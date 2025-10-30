package optimizer

import (
	"github.com/aleksaelezovic/trigo/internal/sparql/parser"
)

// Optimizer optimizes SPARQL queries
type Optimizer struct {
	// Statistics about the data (for selectivity estimation)
	stats *Statistics
}

// Statistics holds statistics about the stored data
type Statistics struct {
	TotalTriples int64
	// Could be extended with per-predicate counts, etc.
}

// NewOptimizer creates a new query optimizer
func NewOptimizer(stats *Statistics) *Optimizer {
	return &Optimizer{
		stats: stats,
	}
}

// Optimize optimizes a parsed query
func (o *Optimizer) Optimize(query *parser.Query) (*OptimizedQuery, error) {
	optimized := &OptimizedQuery{
		Original: query,
	}

	switch query.QueryType {
	case parser.QueryTypeSelect:
		plan, err := o.optimizeSelect(query.Select)
		if err != nil {
			return nil, err
		}
		optimized.Plan = plan
	case parser.QueryTypeAsk:
		plan, err := o.optimizeAsk(query.Ask)
		if err != nil {
			return nil, err
		}
		optimized.Plan = plan
	case parser.QueryTypeConstruct:
		plan, err := o.optimizeConstruct(query.Construct)
		if err != nil {
			return nil, err
		}
		optimized.Plan = plan
	}

	return optimized, nil
}

// OptimizedQuery represents an optimized query with execution plan
type OptimizedQuery struct {
	Original *parser.Query
	Plan     QueryPlan
}

// QueryPlan represents an execution plan
type QueryPlan interface {
	planNode()
}

// ScanPlan represents a scan operation
type ScanPlan struct {
	Pattern *parser.TriplePattern
}

func (p *ScanPlan) planNode() {}

// JoinPlan represents a join operation
type JoinPlan struct {
	Left  QueryPlan
	Right QueryPlan
	Type  JoinType
}

func (p *JoinPlan) planNode() {}

// JoinType represents the type of join
type JoinType int

const (
	JoinTypeNestedLoop JoinType = iota
	JoinTypeHashJoin
	JoinTypeMergeJoin
)

// FilterPlan represents a filter operation
type FilterPlan struct {
	Input  QueryPlan
	Filter *parser.Filter
}

func (p *FilterPlan) planNode() {}

// ProjectionPlan represents a projection operation
type ProjectionPlan struct {
	Input     QueryPlan
	Variables []*parser.Variable
}

func (p *ProjectionPlan) planNode() {}

// OrderByPlan represents an ORDER BY operation
type OrderByPlan struct {
	Input   QueryPlan
	OrderBy []*parser.OrderCondition
}

func (p *OrderByPlan) planNode() {}

// LimitPlan represents a LIMIT operation
type LimitPlan struct {
	Input QueryPlan
	Limit int
}

func (p *LimitPlan) planNode() {}

// OffsetPlan represents an OFFSET operation
type OffsetPlan struct {
	Input  QueryPlan
	Offset int
}

func (p *OffsetPlan) planNode() {}

// DistinctPlan represents a DISTINCT operation
type DistinctPlan struct {
	Input QueryPlan
}

func (p *DistinctPlan) planNode() {}

// ConstructPlan represents a CONSTRUCT operation
type ConstructPlan struct {
	Input    QueryPlan
	Template []*parser.TriplePattern
}

func (p *ConstructPlan) planNode() {}

// GraphPlan represents a GRAPH pattern operation
type GraphPlan struct {
	Input QueryPlan
	Graph *parser.GraphTerm
}

func (p *GraphPlan) planNode() {}

// BindPlan represents a BIND operation (variable assignment)
type BindPlan struct {
	Input      QueryPlan
	Expression parser.Expression
	Variable   *parser.Variable
}

func (p *BindPlan) planNode() {}

// OptionalPlan represents an OPTIONAL pattern (left outer join)
type OptionalPlan struct {
	Left  QueryPlan
	Right QueryPlan
}

func (p *OptionalPlan) planNode() {}

// UnionPlan represents a UNION pattern (alternation)
type UnionPlan struct {
	Left  QueryPlan
	Right QueryPlan
}

func (p *UnionPlan) planNode() {}

// MinusPlan represents a MINUS pattern (set difference)
type MinusPlan struct {
	Left  QueryPlan
	Right QueryPlan
}

func (p *MinusPlan) planNode() {}

// optimizeSelect optimizes a SELECT query
func (o *Optimizer) optimizeSelect(query *parser.SelectQuery) (QueryPlan, error) {
	// Start with the WHERE clause
	plan, err := o.optimizeGraphPattern(query.Where)
	if err != nil {
		return nil, err
	}

	// Apply ORDER BY if present
	if len(query.OrderBy) > 0 {
		plan = &OrderByPlan{
			Input:   plan,
			OrderBy: query.OrderBy,
		}
	}

	// Apply DISTINCT if present
	if query.Distinct {
		plan = &DistinctPlan{
			Input: plan,
		}
	}

	// Apply projection (if not SELECT *)
	if query.Variables != nil {
		plan = &ProjectionPlan{
			Input:     plan,
			Variables: query.Variables,
		}
	}

	// Apply OFFSET if present
	if query.Offset != nil {
		plan = &OffsetPlan{
			Input:  plan,
			Offset: *query.Offset,
		}
	}

	// Apply LIMIT if present
	if query.Limit != nil {
		plan = &LimitPlan{
			Input: plan,
			Limit: *query.Limit,
		}
	}

	return plan, nil
}

// optimizeAsk optimizes an ASK query
func (o *Optimizer) optimizeAsk(query *parser.AskQuery) (QueryPlan, error) {
	// ASK queries just need to check existence
	plan, err := o.optimizeGraphPattern(query.Where)
	if err != nil {
		return nil, err
	}

	// Add implicit LIMIT 1 for ASK queries
	return &LimitPlan{
		Input: plan,
		Limit: 1,
	}, nil
}

// optimizeConstruct optimizes a CONSTRUCT query
func (o *Optimizer) optimizeConstruct(query *parser.ConstructQuery) (QueryPlan, error) {
	// Optimize the WHERE clause to get bindings
	plan, err := o.optimizeGraphPattern(query.Where)
	if err != nil {
		return nil, err
	}

	// Wrap in a ConstructPlan that will apply the template
	return &ConstructPlan{
		Input:    plan,
		Template: query.Template,
	}, nil
}

// optimizeGraphPattern optimizes a graph pattern
func (o *Optimizer) optimizeGraphPattern(pattern *parser.GraphPattern) (QueryPlan, error) {
	switch pattern.Type {
	case parser.GraphPatternTypeBasic:
		return o.optimizeBasicGraphPattern(pattern)
	case parser.GraphPatternTypeGraph:
		return o.optimizeGraphGraphPattern(pattern)
	default:
		// TODO: Handle other pattern types (UNION, OPTIONAL, etc.)
		return o.optimizeBasicGraphPattern(pattern)
	}
}

// optimizeGraphGraphPattern optimizes a GRAPH pattern
func (o *Optimizer) optimizeGraphGraphPattern(pattern *parser.GraphPattern) (QueryPlan, error) {
	// Optimize the nested patterns within the graph
	innerPlan, err := o.optimizeBasicGraphPattern(pattern)
	if err != nil {
		return nil, err
	}

	// Wrap in a GraphPlan that specifies which graph to query
	return &GraphPlan{
		Input: innerPlan,
		Graph: pattern.Graph,
	}, nil
}

// optimizeBasicGraphPattern optimizes a basic graph pattern
func (o *Optimizer) optimizeBasicGraphPattern(pattern *parser.GraphPattern) (QueryPlan, error) {
	var plan QueryPlan

	// Handle triple patterns if present
	if len(pattern.Patterns) > 0 {
		// Reorder triple patterns by selectivity (greedy approach)
		orderedPatterns := o.reorderBySelectivity(pattern.Patterns)

		// Build join plan from ordered patterns
		plan = &ScanPlan{Pattern: orderedPatterns[0]}

		for i := 1; i < len(orderedPatterns); i++ {
			rightPlan := &ScanPlan{Pattern: orderedPatterns[i]}

			// Decide join type based on estimated cost
			joinType := o.selectJoinType(plan, rightPlan)

			plan = &JoinPlan{
				Left:  plan,
				Right: rightPlan,
				Type:  joinType,
			}
		}
	}

	// Handle child patterns (e.g., GRAPH, OPTIONAL, UNION, MINUS patterns)
	for _, child := range pattern.Children {
		childPlan, err := o.optimizeGraphPattern(child)
		if err != nil {
			return nil, err
		}

		if childPlan != nil {
			if plan == nil {
				plan = childPlan
			} else {
				// Create appropriate plan based on child pattern type
				switch child.Type {
				case parser.GraphPatternTypeOptional:
					plan = &OptionalPlan{
						Left:  plan,
						Right: childPlan,
					}
				case parser.GraphPatternTypeUnion:
					plan = &UnionPlan{
						Left:  plan,
						Right: childPlan,
					}
				case parser.GraphPatternTypeMinus:
					plan = &MinusPlan{
						Left:  plan,
						Right: childPlan,
					}
				default:
					// Regular join for other pattern types
					plan = &JoinPlan{
						Left:  plan,
						Right: childPlan,
						Type:  JoinTypeNestedLoop,
					}
				}
			}
		}
	}

	// Apply filters (filter push-down)
	for _, filter := range pattern.Filters {
		if plan != nil {
			plan = &FilterPlan{
				Input:  plan,
				Filter: filter,
			}
		}
	}

	// Apply BIND operations
	for _, bind := range pattern.Binds {
		if plan != nil {
			plan = &BindPlan{
				Input:      plan,
				Expression: bind.Expression,
				Variable:   bind.Variable,
			}
		}
	}

	return plan, nil
}

// reorderBySelectivity reorders triple patterns by estimated selectivity
// More selective patterns (fewer results) should be executed first
func (o *Optimizer) reorderBySelectivity(patterns []*parser.TriplePattern) []*parser.TriplePattern {
	// Create a copy to avoid modifying the original
	ordered := make([]*parser.TriplePattern, len(patterns))
	copy(ordered, patterns)

	// Simple heuristic-based ordering:
	// 1. Patterns with more bound terms are more selective
	// 2. Patterns with bound subjects/predicates are preferred
	for i := 0; i < len(ordered); i++ {
		for j := i + 1; j < len(ordered); j++ {
			if o.estimateSelectivity(ordered[j]) < o.estimateSelectivity(ordered[i]) {
				ordered[i], ordered[j] = ordered[j], ordered[i]
			}
		}
	}

	return ordered
}

// estimateSelectivity estimates the selectivity of a triple pattern
// Lower values indicate higher selectivity (fewer results)
func (o *Optimizer) estimateSelectivity(pattern *parser.TriplePattern) float64 {
	selectivity := 1.0

	// Bound subject is highly selective
	if !pattern.Subject.IsVariable() {
		selectivity *= 0.01
	}

	// Bound predicate is moderately selective
	if !pattern.Predicate.IsVariable() {
		selectivity *= 0.1
	}

	// Bound object is moderately selective
	if !pattern.Object.IsVariable() {
		selectivity *= 0.1
	}

	return selectivity
}

// selectJoinType selects the appropriate join type based on the plans
func (o *Optimizer) selectJoinType(left, right QueryPlan) JoinType {
	// Simple heuristic: use hash join for larger inputs, nested loop for smaller
	// In a real implementation, this would consider statistics and cardinality estimates

	// For now, default to nested loop (simpler to implement)
	return JoinTypeNestedLoop
}

