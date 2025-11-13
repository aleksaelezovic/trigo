package rdf

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// RDFXMLParser parses RDF/XML format
// Note: This is a simplified parser that handles common RDF/XML patterns.
// It supports:
// - rdf:Description elements
// - Properties as XML elements
// - rdf:about, rdf:resource, rdf:ID, rdf:nodeID attributes
// - rdf:datatype, xml:lang attributes
// - Nested blank nodes
// - RDF containers (rdf:Bag, rdf:Seq, rdf:Alt)
// - rdf:li auto-numbering
// - xml:base for base URI resolution
//
// Not yet supported:
// - rdf:parseType="Collection"
type RDFXMLParser struct {
	baseURIStack []string              // Stack of xml:base values
	documentBase string                // Document base URI (file location)
	usedIDs      map[string]bool       // Track used rdf:ID values to detect duplicates
	nodeIDMap    map[string]*BlankNode // Track rdf:nodeID to blank node mappings
	namespaces   map[string]string     // Track in-scope namespace declarations (prefix -> URI)
}

// NewRDFXMLParser creates a new RDF/XML parser
func NewRDFXMLParser() *RDFXMLParser {
	return &RDFXMLParser{
		usedIDs:    make(map[string]bool),
		nodeIDMap:  make(map[string]*BlankNode),
		namespaces: make(map[string]string),
	}
}

// SetBaseURI sets the document base URI (used for resolving relative URIs and rdf:ID)
func (p *RDFXMLParser) SetBaseURI(base string) {
	// Strip fragment from base URI per RFC 3986 section 5.1
	if idx := strings.Index(base, "#"); idx != -1 {
		base = base[:idx]
	}
	p.documentBase = base
}

const (
	rdfNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	itsNS = "http://www.w3.org/2005/11/its"
)

// Forbidden RDF names that cannot be used as node elements
var forbiddenNodeElements = map[string]bool{
	"RDF":             true,
	"ID":              true,
	"about":           true,
	"bagID":           true, // Removed from RDF 1.1
	"parseType":       true,
	"resource":        true,
	"nodeID":          true,
	"datatype":        true,
	"aboutEach":       true, // Removed from RDF 1.1
	"aboutEachPrefix": true, // Removed from RDF 1.1
	"li":              true, // rdf:li cannot be used as typed node element
}

// Forbidden RDF names that cannot be used as property elements
var forbiddenPropertyElements = map[string]bool{
	"Description":     true,
	"RDF":             true,
	"ID":              true,
	"about":           true,
	"bagID":           true, // Removed from RDF 1.1
	"parseType":       true,
	"resource":        true,
	"nodeID":          true,
	"datatype":        true,
	"aboutEach":       true,
	"aboutEachPrefix": true,
	"li":              false, // li is allowed as property element
}

// pushBase adds a base URI to the stack (resolving it against the current base)
func (p *RDFXMLParser) pushBase(base string) {
	// Resolve the new base against the current base
	resolvedBase := p.resolveURI(base)

	// Strip fragment from base URI per RFC 3986 section 5.1
	// Fragments should not be part of the base URI for resolution
	if idx := strings.Index(resolvedBase, "#"); idx != -1 {
		resolvedBase = resolvedBase[:idx]
	}

	p.baseURIStack = append(p.baseURIStack, resolvedBase)
}

// popBase removes the most recent base URI from the stack
func (p *RDFXMLParser) popBase() {
	if len(p.baseURIStack) > 0 {
		p.baseURIStack = p.baseURIStack[:len(p.baseURIStack)-1]
	}
}

// getCurrentBase returns the current base URI (xml:base takes precedence, then document base)
func (p *RDFXMLParser) getCurrentBase() string {
	if len(p.baseURIStack) > 0 {
		return p.baseURIStack[len(p.baseURIStack)-1]
	}
	return p.documentBase
}

// resolveURI resolves a potentially relative URI against the current base URI
func (p *RDFXMLParser) resolveURI(uri string) string {
	base := p.getCurrentBase()
	if base == "" {
		return uri
	}

	// If URI is already absolute, return as-is to preserve original encoding
	// (whether it has Unicode characters or percent-encoding)
	if isAbsoluteURI(uri) {
		return uri
	}

	// Parse the base URI
	baseURL, err := url.Parse(base)
	if err != nil {
		// If base is invalid, return uri as-is
		return uri
	}

	// Parse the URI to resolve
	refURL, err := url.Parse(uri)
	if err != nil {
		// If uri is invalid, return as-is
		return uri
	}

	// Resolve the reference against the base
	resolved := baseURL.ResolveReference(refURL)
	resolvedStr := resolved.String()

	// Decode percent-encoded UTF-8 sequences to preserve Unicode characters
	// This handles the case where XML entities (like &#xFC;) were decoded to Unicode
	// by the XML parser, but then got percent-encoded during URL resolution
	if decoded, err := url.PathUnescape(resolvedStr); err == nil {
		return decoded
	}
	return resolvedStr
}

// isAbsoluteURI checks if a URI is absolute (has a scheme)
func isAbsoluteURI(uri string) bool {
	// Quick check: absolute URIs have scheme:
	// Must start with alpha and contain ':'
	if len(uri) == 0 {
		return false
	}

	// Find the first ':' or invalid character
	for i, c := range uri {
		if c == ':' {
			return i > 0 // Has scheme if ':' is not first character
		}
		// Stop at first character that can't be in a scheme
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(i > 0 && ((c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.'))) {
			return false
		}
	}
	return false
}

// isXMLNCNameStartChar checks if a rune can start an XML NCName (Name without colons)
// Used for rdf:ID validation which must be NCNames
func isXMLNCNameStartChar(r rune) bool {
	return r == '_' ||
		(r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= 0xC0 && r <= 0xD6) ||
		(r >= 0xD8 && r <= 0xF6) ||
		(r >= 0xF8 && r <= 0x2FF) ||
		(r >= 0x370 && r <= 0x37D) ||
		(r >= 0x37F && r <= 0x1FFF) ||
		(r >= 0x200C && r <= 0x200D) ||
		(r >= 0x2070 && r <= 0x218F) ||
		(r >= 0x2C00 && r <= 0x2FEF) ||
		(r >= 0x3001 && r <= 0xD7FF) ||
		(r >= 0xF900 && r <= 0xFDCF) ||
		(r >= 0xFDF0 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0xEFFFF)
}

// isXMLNCNameChar checks if a rune can be part of an XML NCName (no colons allowed)
func isXMLNCNameChar(r rune) bool {
	return isXMLNCNameStartChar(r) ||
		r == '-' || r == '.' ||
		(r >= '0' && r <= '9') ||
		r == 0xB7 ||
		(r >= 0x0300 && r <= 0x036F) ||
		(r >= 0x203F && r <= 0x2040)
}

// isValidXMLNCName checks if a string is a valid XML NCName (no colons)
// This is required for rdf:ID values
func isValidXMLNCName(s string) bool {
	if len(s) == 0 {
		return false
	}

	runes := []rune(s)
	if !isXMLNCNameStartChar(runes[0]) {
		return false
	}

	for i := 1; i < len(runes); i++ {
		if !isXMLNCNameChar(runes[i]) {
			return false
		}
	}

	return true
}

// generateReificationQuads generates the 4 reification quads for a statement
// when rdf:ID is present on a property element
func generateReificationQuads(statementID string, subject, predicate, object Term) []*Quad {
	statementNode := NewNamedNode(statementID)
	rdfType := NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#type")
	rdfStatement := NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#Statement")
	rdfSubject := NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#subject")
	rdfPredicate := NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#predicate")
	rdfObject := NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#object")

	return []*Quad{
		NewQuad(statementNode, rdfType, rdfStatement, NewDefaultGraph()),
		NewQuad(statementNode, rdfSubject, subject, NewDefaultGraph()),
		NewQuad(statementNode, rdfPredicate, predicate, NewDefaultGraph()),
		NewQuad(statementNode, rdfObject, object, NewDefaultGraph()),
	}
}

// resolveID resolves an rdf:ID value against the current base
// It also validates that the ID is a valid XML NCName (no colons) and checks for duplicates
func (p *RDFXMLParser) resolveID(id string) (string, error) {
	// rdf:ID values must be valid XML NCNames (no colons allowed)
	if !isValidXMLNCName(id) {
		return "", fmt.Errorf("rdf:ID value '%s' is not a valid XML NCName (must not contain colons)", id)
	}

	// Resolve the ID to a full URI
	base := p.getCurrentBase()
	var resolvedURI string
	if base != "" {
		resolvedURI = base + "#" + id
	} else {
		resolvedURI = "#" + id
	}

	// Check for duplicate resolved URI within the document
	// (same ID with different xml:base contexts should be allowed)
	if p.usedIDs[resolvedURI] {
		return "", fmt.Errorf("rdf:ID value '%s' is not unique within the document", id)
	}
	p.usedIDs[resolvedURI] = true

	return resolvedURI, nil
}

// getOrCreateNodeID gets or creates a blank node for the given rdf:nodeID
// It validates that the nodeID is a valid XML NCName and maintains consistent mapping
func (p *RDFXMLParser) getOrCreateNodeID(nodeID string, blankNodeCounter *int) (Term, error) {
	// rdf:nodeID values must be valid XML NCNames (no colons allowed)
	if !isValidXMLNCName(nodeID) {
		return nil, fmt.Errorf("rdf:nodeID value '%s' is not a valid XML NCName (must not contain colons)", nodeID)
	}

	// Check if we've already created a blank node for this nodeID
	if node, exists := p.nodeIDMap[nodeID]; exists {
		return node, nil
	}

	// Create a new blank node for this nodeID
	*blankNodeCounter++
	blankNode := NewBlankNode(fmt.Sprintf("b%d", *blankNodeCounter))
	p.nodeIDMap[nodeID] = blankNode
	return blankNode, nil
}

// isContainer checks if an element is an RDF container
func isContainer(elem xml.StartElement) bool {
	if elem.Name.Space != rdfNS {
		return false
	}
	return elem.Name.Local == "Bag" || elem.Name.Local == "Seq" || elem.Name.Local == "Alt"
}

// validateNodeElement checks if an element can be used as a node element
func validateNodeElement(elem xml.StartElement) error {
	if elem.Name.Space == rdfNS {
		if forbiddenNodeElements[elem.Name.Local] {
			return fmt.Errorf("forbidden node element: rdf:%s", elem.Name.Local)
		}
	}
	return nil
}

// validatePropertyElement checks if an element can be used as a property element
func validatePropertyElement(elem xml.StartElement) error {
	if elem.Name.Space == rdfNS {
		if forbidden, exists := forbiddenPropertyElements[elem.Name.Local]; exists && forbidden {
			return fmt.Errorf("forbidden property element: rdf:%s", elem.Name.Local)
		}
	}
	return nil
}

// validateAttributes checks for invalid attribute combinations
func validateAttributes(elem xml.StartElement) error {
	hasAbout := getAttr(elem.Attr, rdfNS, "about") != ""
	hasID := getAttr(elem.Attr, rdfNS, "ID") != ""
	hasResource := getAttr(elem.Attr, rdfNS, "resource") != ""
	hasNodeID := getAttr(elem.Attr, rdfNS, "nodeID") != ""
	parseType := getAttr(elem.Attr, rdfNS, "parseType")

	// Can't have both rdf:about and rdf:ID
	if hasAbout && hasID {
		return fmt.Errorf("element cannot have both rdf:about and rdf:ID")
	}

	// Can't have both rdf:about and rdf:nodeID
	if hasAbout && hasNodeID {
		return fmt.Errorf("element cannot have both rdf:about and rdf:nodeID")
	}

	// Can't have both rdf:ID and rdf:nodeID on NODE elements
	// (but this combination is VALID on property elements - nodeID provides object, ID triggers reification)
	// Since this function is also called for property elements, we skip this check here
	// and let the RDF/XML spec rules handle it naturally

	// Can't have both rdf:ID and rdf:resource on NODE elements
	// (but this combination is VALID on property elements - it creates a triple AND reifies it)
	// Since this function is also called for property elements, we skip this check here
	// and let the RDF/XML spec rules handle it naturally

	// Can't have both rdf:nodeID and rdf:resource on NODE elements
	// (but this combination is VALID on property elements - both specify the object)
	// Since this function is also called for property elements, we skip this check here
	// and let the RDF/XML spec rules handle it naturally

	// Can't have rdf:parseType with rdf:resource
	if parseType != "" && hasResource {
		return fmt.Errorf("rdf:parseType and rdf:resource cannot be used together")
	}

	// Check for rdf:li as an attribute (containers error)
	if getAttr(elem.Attr, rdfNS, "li") != "" {
		return fmt.Errorf("rdf:li cannot be used as an attribute")
	}

	// Check for aboutEach and aboutEachPrefix (removed in RDF 1.1)
	if getAttr(elem.Attr, rdfNS, "aboutEach") != "" {
		return fmt.Errorf("rdf:aboutEach is not supported (removed in RDF 1.1)")
	}
	if getAttr(elem.Attr, rdfNS, "aboutEachPrefix") != "" {
		return fmt.Errorf("rdf:aboutEachPrefix is not supported (removed in RDF 1.1)")
	}

	// Check for bagID (removed in RDF 1.1)
	if getAttr(elem.Attr, rdfNS, "bagID") != "" {
		return fmt.Errorf("rdf:bagID is not supported (removed in RDF 1.1)")
	}

	return nil
}

// Parse parses RDF/XML and returns quads (all in default graph)
func (p *RDFXMLParser) Parse(reader io.Reader) ([]*Quad, error) {
	decoder := xml.NewDecoder(reader)
	var quads []*Quad
	var currentSubject Term
	var blankNodeCounter int
	var liCounter int    // Counter for rdf:li elements within current container
	var seenRootRDF bool // Track if we've seen the root rdf:RDF element

	// Stack to track element depth and inherited properties
	type elementInfo struct {
		name    xml.Name
		hasBase bool
		lang    string // xml:lang value (inherited)
		dir     string // its:dir value (inherited)
		hasLang bool   // whether this element defines xml:lang
		hasDir  bool   // whether this element defines its:dir
	}
	var elementStack []elementInfo

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("XML parse error: %w", err)
		}

		switch elem := token.(type) {
		case xml.StartElement:
			// Capture namespace declarations for XML Literals (C14N requirement)
			for _, attr := range elem.Attr {
				if attr.Name.Space == "xmlns" {
					// xmlns:prefix="uri"
					p.namespaces[attr.Name.Local] = attr.Value
				} else if attr.Name.Space == "" && attr.Name.Local == "xmlns" {
					// xmlns="uri" (default namespace)
					p.namespaces[""] = attr.Value
				}
			}

			// Check for xml:base attribute and push to stack
			xmlBase := getAttrAny(elem.Attr, "base")
			hasBase := xmlBase != ""
			if hasBase {
				p.pushBase(xmlBase)
			}

			// Check for xml:lang attribute
			xmlLang := getAttrAny(elem.Attr, "lang")
			hasLang := xmlLang != ""

			// Check for its:dir attribute
			itsDir := getAttr(elem.Attr, itsNS, "dir")
			hasDir := itsDir != ""

			// Inherit lang/dir from parent if not defined on this element
			parentLang := ""
			parentDir := ""
			if len(elementStack) > 0 {
				parent := elementStack[len(elementStack)-1]
				parentLang = parent.lang
				parentDir = parent.dir
			}
			if !hasLang {
				xmlLang = parentLang
			}
			if !hasDir {
				itsDir = parentDir
			}

			// Track this element on the stack
			elementStack = append(elementStack, elementInfo{
				name:    elem.Name,
				hasBase: hasBase,
				lang:    xmlLang,
				dir:     itsDir,
				hasLang: hasLang,
				hasDir:  hasDir,
			})

			// Check if this is rdf:RDF element
			// The root rdf:RDF is expected and should be skipped
			// But nested rdf:RDF elements are forbidden as node elements
			if elem.Name.Local == "RDF" && elem.Name.Space == rdfNS {
				if !seenRootRDF {
					// First rdf:RDF - this is the root, skip it
					seenRootRDF = true
					continue
				}
				// Nested rdf:RDF - this is forbidden, validate it to get the error
				if err := validateNodeElement(elem); err != nil {
					return nil, fmt.Errorf("invalid RDF/XML: %w", err)
				}
				// Validation passed but nested rdf:RDF should always fail - this shouldn't happen
				return nil, fmt.Errorf("invalid RDF/XML: nested rdf:RDF is forbidden")
			}

			// Check if this is an RDF container (rdf:Bag, rdf:Seq, rdf:Alt)
			// Containers are only treated as node elements if they don't have rdf:resource
			// (if they have rdf:resource, they're property elements)
			resourceAttr := getAttr(elem.Attr, rdfNS, "resource")
			if isContainer(elem) && resourceAttr == "" && currentSubject == nil {
				// Validate attributes
				if err := validateAttributes(elem); err != nil {
					return nil, fmt.Errorf("invalid RDF/XML: %w", err)
				}

				// Check for rdf:about, rdf:ID, or rdf:nodeID, otherwise create blank node
				aboutAttr := getAttr(elem.Attr, rdfNS, "about")
				idAttr := getAttr(elem.Attr, rdfNS, "ID")
				nodeIDAttr := getAttr(elem.Attr, rdfNS, "nodeID")

				var containerNode Term
				if aboutAttr != "" {
					containerNode = NewNamedNode(p.resolveURI(aboutAttr))
				} else if idAttr != "" {
					resolvedID, err := p.resolveID(idAttr)
					if err != nil {
						return nil, fmt.Errorf("invalid RDF/XML: %w", err)
					}
					containerNode = NewNamedNode(resolvedID)
				} else if nodeIDAttr != "" {
					node, err := p.getOrCreateNodeID(nodeIDAttr, &blankNodeCounter)
					if err != nil {
						return nil, fmt.Errorf("invalid RDF/XML: %w", err)
					}
					containerNode = node
				} else {
					// Create blank node for container
					blankNodeCounter++
					containerNode = NewBlankNode(fmt.Sprintf("b%d", blankNodeCounter))
				}

				// Add rdf:type triple
				containerType := rdfNS + elem.Name.Local
				quad := NewQuad(containerNode, NewNamedNode(rdfNS+"type"), NewNamedNode(containerType), NewDefaultGraph())
				quads = append(quads, quad)

				// Process property attributes on container element
				for _, attr := range elem.Attr {
					// Skip RDF-specific attributes (these have special meaning)
					// Skip structural RDF attributes (these have special meaning)
					if attr.Name.Space == rdfNS {
						// Skip structural attributes but allow rdf:_N, rdf:value, and other RDF properties
						switch attr.Name.Local {
						case "about", "ID", "nodeID", "resource", "parseType", "type", "datatype", "bagID":
							continue
						}
					}
					// Skip XML-specific attributes
					if attr.Name.Space == "http://www.w3.org/XML/1998/namespace" ||
						strings.HasPrefix(attr.Name.Space, "http://www.w3.org/XML/") ||
						(attr.Name.Space == "" && (attr.Name.Local == "lang" || attr.Name.Local == "base")) {
						continue
					}
					if attr.Name.Space == "" {
						continue
					}

					attrPredicate := attr.Name.Space + attr.Name.Local
					attrObject := NewLiteral(attr.Value)
					attrQuad := NewQuad(containerNode, NewNamedNode(attrPredicate), attrObject, NewDefaultGraph())
					quads = append(quads, attrQuad)
				}

				// Parse container contents
				liCounter = 0 // Reset counter for this container
				containerQuads, err := p.parseContainer(decoder, containerNode, &liCounter, &blankNodeCounter)
				if err != nil {
					return nil, err
				}
				quads = append(quads, containerQuads...)
				continue
			}

			// Check if this is rdf:Description (subject)
			if elem.Name.Local == "Description" && elem.Name.Space == rdfNS && currentSubject == nil {
				// Validate attributes
				if err := validateAttributes(elem); err != nil {
					return nil, fmt.Errorf("invalid RDF/XML: %w", err)
				}

				// Get rdf:about, rdf:ID, rdf:nodeID, or create blank node
				aboutAttr := getAttr(elem.Attr, rdfNS, "about")
				idAttr := getAttr(elem.Attr, rdfNS, "ID")
				nodeIDAttr := getAttr(elem.Attr, rdfNS, "nodeID")

				// Validate that we don't have conflicting subject specifiers
				if idAttr != "" && nodeIDAttr != "" {
					return nil, fmt.Errorf("invalid RDF/XML: element cannot have both rdf:ID and rdf:nodeID")
				}

				// Check if rdf:about attribute exists (even if empty)
				hasAboutAttr := false
				for _, attr := range elem.Attr {
					if attr.Name.Space == rdfNS && attr.Name.Local == "about" {
						hasAboutAttr = true
						break
					}
				}
				if hasAboutAttr {
					// Empty rdf:about="" resolves to current base URI
					if aboutAttr == "" {
						currentSubject = NewNamedNode(p.getCurrentBase())
					} else {
						currentSubject = NewNamedNode(p.resolveURI(aboutAttr))
					}
				} else if idAttr != "" {
					resolvedID, err := p.resolveID(idAttr)
					if err != nil {
						return nil, fmt.Errorf("invalid RDF/XML: %w", err)
					}
					currentSubject = NewNamedNode(resolvedID)
				} else if nodeIDAttr != "" {
					node, err := p.getOrCreateNodeID(nodeIDAttr, &blankNodeCounter)
					if err != nil {
						return nil, fmt.Errorf("invalid RDF/XML: %w", err)
					}
					currentSubject = node
				} else {
					// Blank node
					blankNodeCounter++
					currentSubject = NewBlankNode(fmt.Sprintf("b%d", blankNodeCounter))
				}

				// Reset liCounter for this Description element
				liCounter = 0

				// Process property attributes on Description element
				// Get inherited xml:lang and its:dir from element stack
				currentLang := ""
				currentDir := ""
				if len(elementStack) > 0 {
					currentElem := elementStack[len(elementStack)-1]
					currentLang = currentElem.lang
					currentDir = currentElem.dir
				}

				for _, attr := range elem.Attr {
					// Skip structural RDF attributes (these have special meaning)
					if attr.Name.Space == rdfNS {
						// Only skip these specific structural attributes
						if attr.Name.Local == "about" || attr.Name.Local == "ID" ||
							attr.Name.Local == "nodeID" || attr.Name.Local == "resource" ||
							attr.Name.Local == "datatype" || attr.Name.Local == "parseType" {
							continue
						}
						// Allow rdf:type and other RDF namespace attributes as properties
					}
					if attr.Name.Local == "base" || attr.Name.Local == "lang" {
						// Skip xml:base and xml:lang
						continue
					}
					if attr.Name.Space == itsNS {
						// Skip ITS attributes (its:dir, its:version, etc.)
						continue
					}
					if attr.Name.Space == "" {
						// Skip attributes without namespace
						continue
					}

					// This is a property attribute
					predicate := attr.Name.Space + attr.Name.Local

					// Create object - rdf:type takes an IRI, others take literals
					var object Term
					if attr.Name.Space == rdfNS && attr.Name.Local == "type" {
						// rdf:type value should be resolved as an IRI
						object = NewNamedNode(p.resolveURI(attr.Value))
					} else if currentLang != "" {
						// Language-tagged literal with optional direction
						object = &Literal{
							Value:     attr.Value,
							Language:  currentLang,
							Direction: currentDir,
						}
					} else {
						// Plain literal
						object = NewLiteral(attr.Value)
					}

					quad := NewQuad(currentSubject, NewNamedNode(predicate), object, NewDefaultGraph())
					quads = append(quads, quad)
				}

				continue
			}

			// Check if this is a typed node (not rdf:Description but has rdf:about/ID/nodeID)
			aboutAttr := getAttr(elem.Attr, rdfNS, "about")
			idAttr := getAttr(elem.Attr, rdfNS, "ID")
			nodeIDAttr := getAttr(elem.Attr, rdfNS, "nodeID")
			// Check if rdf:about attribute exists (even if empty)
			hasAboutAttr := false
			for _, attr := range elem.Attr {
				if attr.Name.Space == rdfNS && attr.Name.Local == "about" {
					hasAboutAttr = true
					break
				}
			}
			// Treat as node element ONLY if: has rdf:about, OR currentSubject is nil, OR (has rdf:ID/nodeID and currentSubject is nil)
			// rdf:ID/nodeID on property elements (when currentSubject != nil) trigger reification, not node elements
			if hasAboutAttr || currentSubject == nil {
				// Validate this is not a forbidden node element
				if err := validateNodeElement(elem); err != nil {
					return nil, fmt.Errorf("invalid RDF/XML: %w", err)
				}

				// Validate attributes
				if err := validateAttributes(elem); err != nil {
					return nil, fmt.Errorf("invalid RDF/XML: %w", err)
				}

				// This is a typed node (implicit rdf:type)
				var subject Term
				if hasAboutAttr {
					// Empty rdf:about="" resolves to current base URI
					if aboutAttr == "" {
						subject = NewNamedNode(p.getCurrentBase())
					} else {
						subject = NewNamedNode(p.resolveURI(aboutAttr))
					}
				} else if idAttr != "" {
					resolvedID, err := p.resolveID(idAttr)
					if err != nil {
						return nil, fmt.Errorf("invalid RDF/XML: %w", err)
					}
					subject = NewNamedNode(resolvedID)
				} else if nodeIDAttr != "" {
					node, err := p.getOrCreateNodeID(nodeIDAttr, &blankNodeCounter)
					if err != nil {
						return nil, fmt.Errorf("invalid RDF/XML: %w", err)
					}
					subject = node
				} else {
					blankNodeCounter++
					subject = NewBlankNode(fmt.Sprintf("b%d", blankNodeCounter))
				}

				// Add rdf:type triple
				nodeType := elem.Name.Space + elem.Name.Local
				quad := NewQuad(subject, NewNamedNode(rdfNS+"type"), NewNamedNode(nodeType), NewDefaultGraph())
				quads = append(quads, quad)

				// Parse properties of this typed node
				liCounter = 0 // Reset counter
				typedNodeQuads, err := p.parseTypedNode(decoder, subject, &liCounter, &blankNodeCounter)
				if err != nil {
					return nil, err
				}
				quads = append(quads, typedNodeQuads...)
				continue
			}

			// This is a property element
			if currentSubject != nil {
				// Validate property element
				if err := validatePropertyElement(elem); err != nil {
					return nil, fmt.Errorf("invalid RDF/XML: %w", err)
				}

				// Validate attributes (for illegal combinations)
				if err := validateAttributes(elem); err != nil {
					return nil, err
				}

				// Handle rdf:li specially - it auto-numbers to rdf:_N
				var predicate string
				if elem.Name.Local == "li" && elem.Name.Space == rdfNS {
					liCounter++
					predicate = fmt.Sprintf("%s_%d", rdfNS, liCounter)
				} else {
					predicate = elem.Name.Space + elem.Name.Local
				}

				// Check for rdf:parseType (highest priority)
				parseTypeAttr := getAttr(elem.Attr, rdfNS, "parseType")
				if parseTypeAttr != "" {
					// Use parsePropertyContent to handle parseType
					object, nestedQuads, err := p.parsePropertyContent(decoder, elem, &blankNodeCounter)
					if err != nil {
						return nil, err
					}

					quad := NewQuad(currentSubject, NewNamedNode(predicate), object, NewDefaultGraph())
					quads = append(quads, quad)
					quads = append(quads, nestedQuads...)

					// Check for rdf:ID on property element (triggers reification)
					if idAttr := getAttr(elem.Attr, rdfNS, "ID"); idAttr != "" {
						statementID, err := p.resolveID(idAttr)
						if err != nil {
							return nil, fmt.Errorf("invalid RDF/XML: %w", err)
						}
						reificationQuads := generateReificationQuads(statementID, currentSubject, NewNamedNode(predicate), object)
						quads = append(quads, reificationQuads...)
					}

					continue
				}

				// Validate that rdf:resource and rdf:nodeID aren't both present
				if getAttr(elem.Attr, rdfNS, "resource") != "" && getAttr(elem.Attr, rdfNS, "nodeID") != "" {
					return nil, fmt.Errorf("invalid RDF/XML: element cannot have both rdf:resource and rdf:nodeID")
				}

				// Check for rdf:resource attribute (second priority)
				resourceAttr := getAttr(elem.Attr, rdfNS, "resource")
				if resourceAttr != "" {
					object := NewNamedNode(p.resolveURI(resourceAttr))

					// Check for non-RDF property attributes - these apply to the resource IRI
					hasPropertyAttrs := false
					for _, attr := range elem.Attr {
						// Skip RDF-specific and XML-specific attributes
						if attr.Name.Space == rdfNS {
							continue
						}
						if attr.Name.Space == itsNS {
							// Skip ITS namespace attributes (its:dir, its:version, etc.)
							continue
						}
						if attr.Name.Space == "http://www.w3.org/XML/1998/namespace" ||
							strings.HasPrefix(attr.Name.Space, "http://www.w3.org/XML/") ||
							(attr.Name.Space == "" && (attr.Name.Local == "lang" || attr.Name.Local == "base")) {
							continue
						}
						if attr.Name.Space == "" {
							continue
						}
						hasPropertyAttrs = true
						break
					}

					if hasPropertyAttrs {
						// Create triples for property attributes on the resource
						for _, attr := range elem.Attr {
							// Skip RDF-specific and XML-specific attributes
							if attr.Name.Space == rdfNS {
								continue
							}
							if attr.Name.Space == itsNS {
								// Skip ITS namespace attributes (its:dir, its:version, etc.)
								continue
							}
							if attr.Name.Space == "http://www.w3.org/XML/1998/namespace" ||
								strings.HasPrefix(attr.Name.Space, "http://www.w3.org/XML/") ||
								(attr.Name.Space == "" && (attr.Name.Local == "lang" || attr.Name.Local == "base")) {
								continue
							}
							if attr.Name.Space == "" {
								continue
							}

							attrPredicate := attr.Name.Space + attr.Name.Local
							attrObject := NewLiteral(attr.Value)
							attrQuad := NewQuad(object, NewNamedNode(attrPredicate), attrObject, NewDefaultGraph())
							quads = append(quads, attrQuad)
						}
					}

					// Create main triple
					quad := NewQuad(currentSubject, NewNamedNode(predicate), object, NewDefaultGraph())
					quads = append(quads, quad)

					// Check for rdf:ID on property element (triggers reification)
					if idAttr := getAttr(elem.Attr, rdfNS, "ID"); idAttr != "" {
						statementID, err := p.resolveID(idAttr)
						if err != nil {
							return nil, fmt.Errorf("invalid RDF/XML: %w", err)
						}
						reificationQuads := generateReificationQuads(statementID, currentSubject, NewNamedNode(predicate), object)
						quads = append(quads, reificationQuads...)
					}

					// Consume the end element
					for {
						token, err := decoder.Token()
						if err != nil {
							return nil, fmt.Errorf("error reading property content: %w", err)
						}
						if _, ok := token.(xml.EndElement); ok {
							break
						}
					}

					continue
				}

				// Check for rdf:nodeID attribute (third priority - similar to rdf:resource but for blank nodes)
				nodeIDAttr := getAttr(elem.Attr, rdfNS, "nodeID")
				if nodeIDAttr != "" {
					object, err := p.getOrCreateNodeID(nodeIDAttr, &blankNodeCounter)
					if err != nil {
						return nil, fmt.Errorf("invalid RDF/XML: %w", err)
					}

					// Create main triple
					quad := NewQuad(currentSubject, NewNamedNode(predicate), object, NewDefaultGraph())
					quads = append(quads, quad)

					// Check for rdf:ID on property element (triggers reification)
					if idAttr := getAttr(elem.Attr, rdfNS, "ID"); idAttr != "" {
						statementID, err := p.resolveID(idAttr)
						if err != nil {
							return nil, fmt.Errorf("invalid RDF/XML: %w", err)
						}
						reificationQuads := generateReificationQuads(statementID, currentSubject, NewNamedNode(predicate), object)
						quads = append(quads, reificationQuads...)
					}

					// Consume the end element
					for {
						token, err := decoder.Token()
						if err != nil {
							return nil, fmt.Errorf("error reading property content: %w", err)
						}
						if _, ok := token.(xml.EndElement); ok {
							break
						}
					}

					continue
				}

				// Check for non-RDF property attributes (these create a blank node with additional properties)
				hasPropertyAttrs := false
				for _, attr := range elem.Attr {
					// Skip RDF-specific and XML-specific attributes
					if attr.Name.Space == rdfNS {
						continue
					}
					if attr.Name.Space == itsNS {
						// Skip ITS namespace attributes (its:dir, its:version, etc.)
						continue
					}
					if attr.Name.Space == "http://www.w3.org/XML/1998/namespace" ||
						strings.HasPrefix(attr.Name.Space, "http://www.w3.org/XML/") ||
						(attr.Name.Space == "" && (attr.Name.Local == "lang" || attr.Name.Local == "base")) {
						continue
					}
					if attr.Name.Space == "" {
						continue
					}
					hasPropertyAttrs = true
					break
				}

				if hasPropertyAttrs {
					// Create blank node as object
					blankNodeCounter++
					blankNode := NewBlankNode(fmt.Sprintf("b%d", blankNodeCounter))

					// Create triple with blank node as object
					quad := NewQuad(currentSubject, NewNamedNode(predicate), blankNode, NewDefaultGraph())
					quads = append(quads, quad)

					// Check for rdf:ID on property element (triggers reification)
					if idAttr := getAttr(elem.Attr, rdfNS, "ID"); idAttr != "" {
						statementID, err := p.resolveID(idAttr)
						if err != nil {
							return nil, fmt.Errorf("invalid RDF/XML: %w", err)
						}
						reificationQuads := generateReificationQuads(statementID, currentSubject, NewNamedNode(predicate), blankNode)
						quads = append(quads, reificationQuads...)
					}

					// Create triples for property attributes
					for _, attr := range elem.Attr {
						// Skip RDF-specific and XML-specific attributes
						if attr.Name.Space == rdfNS {
							continue
						}
						if attr.Name.Space == itsNS {
							// Skip ITS namespace attributes (its:dir, its:version, etc.)
							continue
						}
						if attr.Name.Space == "http://www.w3.org/XML/1998/namespace" ||
							strings.HasPrefix(attr.Name.Space, "http://www.w3.org/XML/") ||
							(attr.Name.Space == "" && (attr.Name.Local == "lang" || attr.Name.Local == "base")) {
							continue
						}
						if attr.Name.Space == "" {
							continue
						}

						attrPredicate := attr.Name.Space + attr.Name.Local
						attrObject := NewLiteral(attr.Value)
						attrQuad := NewQuad(blankNode, NewNamedNode(attrPredicate), attrObject, NewDefaultGraph())
						quads = append(quads, attrQuad)
					}

					// Consume the end element
					for {
						token, err := decoder.Token()
						if err != nil {
							return nil, fmt.Errorf("error reading property content: %w", err)
						}
						if _, ok := token.(xml.EndElement); ok {
							break
						}
					}

					continue
				}

				// No special attributes - read the text content or check for empty element
				datatypeAttr := getAttr(elem.Attr, rdfNS, "datatype")
				langAttr := getAttrAny(elem.Attr, "lang")
				dirAttr := getAttr(elem.Attr, itsNS, "dir")

				// Inherit lang/dir from element stack if not defined locally
				if langAttr == "" || dirAttr == "" {
					if len(elementStack) > 0 {
						currentElem := elementStack[len(elementStack)-1]
						if langAttr == "" {
							langAttr = currentElem.lang
						}
						if dirAttr == "" {
							dirAttr = currentElem.dir
						}
					}
				}

				// Capture rdf:ID early before entering nested loops (elem.Attr may not be accessible later)
				propertyIDAttr := getAttr(elem.Attr, rdfNS, "ID")

				// Read the text content
				var textContent strings.Builder
				for {
					token, err := decoder.Token()
					if err != nil {
						return nil, fmt.Errorf("error reading property content: %w", err)
					}

					switch t := token.(type) {
					case xml.CharData:
						textContent.Write(t)
					case xml.EndElement:
						// End of property element
						var object Term
						if datatypeAttr != "" {
							// Typed literal
							object = &Literal{
								Value:    textContent.String(),
								Datatype: NewNamedNode(datatypeAttr),
							}
						} else if langAttr != "" {
							// Language-tagged literal with optional direction
							object = &Literal{
								Value:     textContent.String(),
								Language:  langAttr,
								Direction: dirAttr,
							}
						} else {
							// Plain literal
							object = NewLiteral(textContent.String())
						}

						quad := NewQuad(currentSubject, NewNamedNode(predicate), object, NewDefaultGraph())
						quads = append(quads, quad)

						// Check for rdf:ID on property element (triggers reification)
						if propertyIDAttr != "" {
							statementID, err := p.resolveID(propertyIDAttr)
							if err != nil {
								return nil, fmt.Errorf("invalid RDF/XML: %w", err)
							}
							reificationQuads := generateReificationQuads(statementID, currentSubject, NewNamedNode(predicate), object)
							quads = append(quads, reificationQuads...)
						}

						goto propertyDone
					case xml.StartElement:
						// Nested element (blank node or another Description)
						if t.Name.Local == "Description" && t.Name.Space == rdfNS {
							// Check for rdf:about, rdf:ID, or rdf:nodeID on nested Description
							aboutAttr := getAttr(t.Attr, rdfNS, "about")
							idAttr := getAttr(t.Attr, rdfNS, "ID")
							nodeIDAttr := getAttr(t.Attr, rdfNS, "nodeID")

							var object Term
							if aboutAttr != "" {
								object = NewNamedNode(p.resolveURI(aboutAttr))
							} else if idAttr != "" {
								resolvedID, err := p.resolveID(idAttr)
								if err != nil {
									return nil, fmt.Errorf("invalid RDF/XML: %w", err)
								}
								object = NewNamedNode(resolvedID)
							} else if nodeIDAttr != "" {
								node, err := p.getOrCreateNodeID(nodeIDAttr, &blankNodeCounter)
								if err != nil {
									return nil, fmt.Errorf("invalid RDF/XML: %w", err)
								}
								object = node
							} else {
								// Nested blank node without attributes
								blankNodeCounter++
								object = NewBlankNode(fmt.Sprintf("b%d", blankNodeCounter))
							}

							quad := NewQuad(currentSubject, NewNamedNode(predicate), object, NewDefaultGraph())
							quads = append(quads, quad)

							// Parse nested Description
							nestedQuads, err := p.parseNestedDescription(decoder, object, &blankNodeCounter)
							if err != nil {
								return nil, err
							}
							quads = append(quads, nestedQuads...)

							// Check for rdf:ID on property element (triggers reification)
							if propertyIDAttr != "" {
								statementID, err := p.resolveID(propertyIDAttr)
								if err != nil {
									return nil, fmt.Errorf("invalid RDF/XML: %w", err)
								}
								reificationQuads := generateReificationQuads(statementID, currentSubject, NewNamedNode(predicate), object)
								quads = append(quads, reificationQuads...)
							}
							goto propertyDone
						}
					}
				}
			propertyDone:
			}

		case xml.EndElement:
			// Pop the element from the stack and check if we need to pop xml:base
			if len(elementStack) > 0 {
				info := elementStack[len(elementStack)-1]
				elementStack = elementStack[:len(elementStack)-1]

				if info.hasBase {
					p.popBase()
				}
			}

			// End of rdf:Description - reset current subject
			if elem.Name.Local == "Description" && elem.Name.Space == rdfNS {
				currentSubject = nil
			}
		}
	}

	return quads, nil
}

// parseContainer parses the contents of an RDF container (Bag, Seq, Alt)
func (p *RDFXMLParser) parseContainer(decoder *xml.Decoder, containerNode Term, liCounter *int, blankNodeCounter *int) ([]*Quad, error) {
	var quads []*Quad

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("error parsing container: %w", err)
		}

		switch elem := token.(type) {
		case xml.StartElement:
			// Check if this is rdf:li (auto-numbered member)
			if elem.Name.Local == "li" && elem.Name.Space == rdfNS {
				*liCounter++
				memberPredicate := fmt.Sprintf("%s_%d", rdfNS, *liCounter)

				// Parse the content
				object, nestedQuads, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
				if err != nil {
					return nil, err
				}

				quad := NewQuad(containerNode, NewNamedNode(memberPredicate), object, NewDefaultGraph())
				quads = append(quads, quad)
				quads = append(quads, nestedQuads...)
				continue
			}

			// Check if this is an explicit rdf:_N member
			if strings.HasPrefix(elem.Name.Local, "_") && elem.Name.Space == rdfNS {
				memberPredicate := elem.Name.Space + elem.Name.Local

				// Parse the content
				object, nestedQuads, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
				if err != nil {
					return nil, err
				}

				quad := NewQuad(containerNode, NewNamedNode(memberPredicate), object, NewDefaultGraph())
				quads = append(quads, quad)
				quads = append(quads, nestedQuads...)
				continue
			}

			// Other properties (not typical in containers, but allowed)
			predicate := elem.Name.Space + elem.Name.Local
			object, nestedQuads, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
			if err != nil {
				return nil, err
			}

			quad := NewQuad(containerNode, NewNamedNode(predicate), object, NewDefaultGraph())
			quads = append(quads, quad)
			quads = append(quads, nestedQuads...)

		case xml.EndElement:
			// End of container
			if elem.Name.Space == rdfNS && (elem.Name.Local == "Bag" || elem.Name.Local == "Seq" || elem.Name.Local == "Alt") {
				return quads, nil
			}
		}
	}
}

// parseTypedNode parses properties of a typed node (like <foo:Bar>)
func (p *RDFXMLParser) parseTypedNode(decoder *xml.Decoder, subject Term, liCounter *int, blankNodeCounter *int) ([]*Quad, error) {
	var quads []*Quad

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("error parsing typed node: %w", err)
		}

		switch elem := token.(type) {
		case xml.StartElement:
			// Check if this is rdf:li (for when typed node is also a container)
			if elem.Name.Local == "li" && elem.Name.Space == rdfNS {
				*liCounter++
				memberPredicate := fmt.Sprintf("%s_%d", rdfNS, *liCounter)

				object, nestedQuads, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
				if err != nil {
					return nil, err
				}

				quad := NewQuad(subject, NewNamedNode(memberPredicate), object, NewDefaultGraph())
				quads = append(quads, quad)
				quads = append(quads, nestedQuads...)

				// Check for rdf:ID on property element (triggers reification)
				if idAttr := getAttr(elem.Attr, rdfNS, "ID"); idAttr != "" {
					statementID, err := p.resolveID(idAttr)
					if err != nil {
						return nil, err
					}
					reificationQuads := generateReificationQuads(statementID, subject, NewNamedNode(memberPredicate), object)
					quads = append(quads, reificationQuads...)
				}

				continue
			}

			// Check if this is an explicit rdf:_N member
			if strings.HasPrefix(elem.Name.Local, "_") && elem.Name.Space == rdfNS {
				memberPredicate := elem.Name.Space + elem.Name.Local

				object, nestedQuads, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
				if err != nil {
					return nil, err
				}

				quad := NewQuad(subject, NewNamedNode(memberPredicate), object, NewDefaultGraph())
				quads = append(quads, quad)
				quads = append(quads, nestedQuads...)

				// Check for rdf:ID on property element (triggers reification)
				if idAttr := getAttr(elem.Attr, rdfNS, "ID"); idAttr != "" {
					statementID, err := p.resolveID(idAttr)
					if err != nil {
						return nil, err
					}
					reificationQuads := generateReificationQuads(statementID, subject, NewNamedNode(memberPredicate), object)
					quads = append(quads, reificationQuads...)
				}

				continue
			}

			// Regular property
			predicate := elem.Name.Space + elem.Name.Local
			object, nestedQuads, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
			if err != nil {
				return nil, err
			}

			quad := NewQuad(subject, NewNamedNode(predicate), object, NewDefaultGraph())
			quads = append(quads, quad)
			quads = append(quads, nestedQuads...)

			// Check for rdf:ID on property element (triggers reification)
			if idAttr := getAttr(elem.Attr, rdfNS, "ID"); idAttr != "" {
				statementID, err := p.resolveID(idAttr)
				if err != nil {
					return nil, err
				}
				reificationQuads := generateReificationQuads(statementID, subject, NewNamedNode(predicate), object)
				quads = append(quads, reificationQuads...)
			}

		case xml.EndElement:
			// End of typed node
			return quads, nil
		}
	}
}

// parsePropertyContent parses the content of a property element and returns the object
// It also returns any additional quads generated (e.g., for parseType="Resource")
func (p *RDFXMLParser) parsePropertyContent(decoder *xml.Decoder, elem xml.StartElement, blankNodeCounter *int) (Term, []*Quad, error) {
	// Validate attributes for illegal combinations
	if err := validateAttributes(elem); err != nil {
		return nil, nil, err
	}

	// Check for rdf:parseType attribute
	parseTypeAttr := getAttr(elem.Attr, rdfNS, "parseType")
	if parseTypeAttr == "Resource" {
		// Create a blank node for the resource
		*blankNodeCounter++
		blankNode := NewBlankNode(fmt.Sprintf("b%d", *blankNodeCounter))

		// Parse nested properties as properties of this blank node
		var quads []*Quad
		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, nil, err
			}

			switch t := token.(type) {
			case xml.StartElement:
				// This is a property of the blank node
				predicate := t.Name.Space + t.Name.Local
				object, nestedQuads, err := p.parsePropertyContent(decoder, t, blankNodeCounter)
				if err != nil {
					return nil, nil, err
				}

				quad := NewQuad(blankNode, NewNamedNode(predicate), object, NewDefaultGraph())
				quads = append(quads, quad)
				quads = append(quads, nestedQuads...)

				// Check for rdf:ID on nested property element (triggers reification)
				if idAttr := getAttr(t.Attr, rdfNS, "ID"); idAttr != "" {
					statementID, err := p.resolveID(idAttr)
					if err != nil {
						return nil, nil, fmt.Errorf("invalid RDF/XML: %w", err)
					}
					reificationQuads := generateReificationQuads(statementID, blankNode, NewNamedNode(predicate), object)
					quads = append(quads, reificationQuads...)
				}

			case xml.EndElement:
				// End of parseType="Resource" element
				return blankNode, quads, nil
			}
		}
	} else if parseTypeAttr == "Literal" {
		// Parse content as XML literal with C14N
		// According to RDF/XML spec, we need to include in-scope namespace declarations
		var content strings.Builder
		depth := 1
		firstElement := true // Track if this is the first element to add namespace declarations

		// Use in-scope namespaces from the parser (captured during parsing)
		// This includes namespaces declared on ancestor elements like <rdf:RDF>
		nsDecls := make(map[string]string)
		for prefix, uri := range p.namespaces {
			nsDecls[prefix] = uri
		}

		// Also add any namespace declarations directly on the property element
		for _, attr := range elem.Attr {
			if attr.Name.Space == "xmlns" {
				// xmlns:prefix="uri"
				nsDecls[attr.Name.Local] = attr.Value
			} else if attr.Name.Space == "" && attr.Name.Local == "xmlns" {
				// xmlns="uri" (default namespace)
				nsDecls[""] = attr.Value
			}
		}

		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, nil, err
			}

			switch t := token.(type) {
			case xml.StartElement:
				// Serialize start element
				content.WriteString("<")
				content.WriteString(t.Name.Local)

				// Add in-scope namespace declarations to first element (C14N requirement)
				if firstElement {
					// Write namespace declarations in sorted order for consistency
					prefixes := make([]string, 0, len(nsDecls))
					for prefix := range nsDecls {
						prefixes = append(prefixes, prefix)
					}
					// Simple sort
					for i := 0; i < len(prefixes); i++ {
						for j := i + 1; j < len(prefixes); j++ {
							if prefixes[i] < prefixes[j] {
								prefixes[i], prefixes[j] = prefixes[j], prefixes[i]
							}
						}
					}
					for _, prefix := range prefixes {
						if prefix == "" {
							content.WriteString(" xmlns=\"")
							content.WriteString(nsDecls[prefix])
							content.WriteString("\"")
						} else {
							content.WriteString(" xmlns:")
							content.WriteString(prefix)
							content.WriteString("=\"")
							content.WriteString(nsDecls[prefix])
							content.WriteString("\"")
						}
					}
					firstElement = false
				}

				// Add element's own attributes
				for _, attr := range t.Attr {
					content.WriteString(" ")
					if attr.Name.Space != "" {
						content.WriteString(attr.Name.Space)
						content.WriteString(":")
					}
					content.WriteString(attr.Name.Local)
					content.WriteString("=\"")
					content.WriteString(attr.Value)
					content.WriteString("\"")
				}
				content.WriteString(">")
				depth++

			case xml.EndElement:
				depth--
				if depth == 0 {
					// End of parseType="Literal" element
					xmlLiteral := &Literal{
						Value:    content.String(),
						Datatype: NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#XMLLiteral"),
					}
					return xmlLiteral, nil, nil
				}
				// Serialize end element
				content.WriteString("</")
				if t.Name.Space != "" {
					content.WriteString(t.Name.Local)
				} else {
					content.WriteString(t.Name.Local)
				}
				content.WriteString(">")

			case xml.CharData:
				content.Write(t)
			}
		}
	} else if parseTypeAttr == "Collection" {
		// Parse content as a collection (linked list)
		// Create a chain of blank nodes with rdf:first/rdf:rest
		var quads []*Quad
		var headNode Term
		var currentNode Term

		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, nil, err
			}

			switch t := token.(type) {
			case xml.StartElement:
				// Parse each child element as a collection item
				var itemNode Term

				// Check if it's a Description or typed node
				if t.Name.Local == "Description" && t.Name.Space == rdfNS {
					// Parse as Description to get the subject
					aboutAttr := getAttr(t.Attr, rdfNS, "about")
					idAttr := getAttr(t.Attr, rdfNS, "ID")
					nodeIDAttr := getAttr(t.Attr, rdfNS, "nodeID")

					if aboutAttr != "" {
						itemNode = NewNamedNode(p.resolveURI(aboutAttr))
					} else if idAttr != "" {
						resolvedID, err := p.resolveID(idAttr)
						if err != nil {
							return nil, nil, err
						}
						itemNode = NewNamedNode(resolvedID)
					} else if nodeIDAttr != "" {
						node, err := p.getOrCreateNodeID(nodeIDAttr, blankNodeCounter)
						if err != nil {
							return nil, nil, err
						}
						itemNode = node
					} else {
						*blankNodeCounter++
						itemNode = NewBlankNode(fmt.Sprintf("b%d", *blankNodeCounter))
					}

					// Consume the Description element
					for {
						tok, err := decoder.Token()
						if err != nil {
							return nil, nil, err
						}
						if _, ok := tok.(xml.EndElement); ok {
							break
						}
					}
				} else {
					// Typed node - use its type IRI
					itemNode = NewNamedNode(t.Name.Space + t.Name.Local)
					// Consume the element
					for {
						tok, err := decoder.Token()
						if err != nil {
							return nil, nil, err
						}
						if _, ok := tok.(xml.EndElement); ok {
							break
						}
					}
				}

				// Create a blank node for this list cell
				*blankNodeCounter++
				listCell := NewBlankNode(fmt.Sprintf("b%d", *blankNodeCounter))

				// Add rdf:first triple
				quads = append(quads, NewQuad(listCell, NewNamedNode(rdfNS+"first"), itemNode, NewDefaultGraph()))

				// Link from previous cell or set as head
				if currentNode != nil {
					// Link previous cell's rest to this cell
					quads = append(quads, NewQuad(currentNode, NewNamedNode(rdfNS+"rest"), listCell, NewDefaultGraph()))
				} else {
					// This is the head of the list
					headNode = listCell
				}
				currentNode = listCell

			case xml.EndElement:
				// End of collection
				if currentNode != nil {
					// Terminate the list with rdf:nil
					quads = append(quads, NewQuad(currentNode, NewNamedNode(rdfNS+"rest"), NewNamedNode(rdfNS+"nil"), NewDefaultGraph()))
				} else {
					// Empty collection - return rdf:nil
					return NewNamedNode(rdfNS + "nil"), nil, nil
				}
				return headNode, quads, nil
			}
		}
	}

	// Check for rdf:resource attribute (object is IRI)
	resourceAttr := getAttr(elem.Attr, rdfNS, "resource")
	if resourceAttr != "" {
		// Consume the end element
		_, err := decoder.Token()
		if err != nil {
			return nil, nil, err
		}
		return NewNamedNode(p.resolveURI(resourceAttr)), nil, nil
	}

	// Check for rdf:nodeID attribute (object is blank node)
	nodeIDAttr := getAttr(elem.Attr, rdfNS, "nodeID")
	if nodeIDAttr != "" {
		// Consume the end element
		_, err := decoder.Token()
		if err != nil {
			return nil, nil, err
		}
		node, err := p.getOrCreateNodeID(nodeIDAttr, blankNodeCounter)
		if err != nil {
			return nil, nil, err
		}
		return node, nil, nil
	}

	// Check for non-RDF property attributes (these create a blank node with additional properties)
	hasPropertyAttrs := false
	for _, attr := range elem.Attr {
		// Skip RDF-specific and XML-specific attributes
		if attr.Name.Space == rdfNS {
			continue
		}
		if attr.Name.Space == itsNS {
			// Skip ITS namespace attributes (its:dir, its:version, etc.)
			continue
		}
		if attr.Name.Space == "http://www.w3.org/XML/1998/namespace" ||
			strings.HasPrefix(attr.Name.Space, "http://www.w3.org/XML/") ||
			(attr.Name.Space == "" && (attr.Name.Local == "lang" || attr.Name.Local == "base")) {
			continue
		}
		if attr.Name.Space == "" {
			continue
		}
		hasPropertyAttrs = true
		break
	}

	if hasPropertyAttrs {
		// Create blank node as object
		*blankNodeCounter++
		blankNode := NewBlankNode(fmt.Sprintf("b%d", *blankNodeCounter))

		// Create triples for property attributes
		var quads []*Quad
		for _, attr := range elem.Attr {
			// Skip RDF-specific and XML-specific attributes
			if attr.Name.Space == rdfNS {
				continue
			}
			if attr.Name.Space == itsNS {
				// Skip ITS namespace attributes (its:dir, its:version, etc.)
				continue
			}
			if attr.Name.Space == "http://www.w3.org/XML/1998/namespace" ||
				strings.HasPrefix(attr.Name.Space, "http://www.w3.org/XML/") ||
				(attr.Name.Space == "" && (attr.Name.Local == "lang" || attr.Name.Local == "base")) {
				continue
			}
			if attr.Name.Space == "" {
				continue
			}

			attrPredicate := attr.Name.Space + attr.Name.Local
			attrObject := NewLiteral(attr.Value)
			attrQuad := NewQuad(blankNode, NewNamedNode(attrPredicate), attrObject, NewDefaultGraph())
			quads = append(quads, attrQuad)
		}

		// Consume the end element
		_, err := decoder.Token()
		if err != nil {
			return nil, nil, err
		}

		return blankNode, quads, nil
	}
	// Check for rdf:datatype attribute
	datatypeAttr := getAttr(elem.Attr, rdfNS, "datatype")

	// Check for xml:lang attribute
	langAttr := getAttrAny(elem.Attr, "lang")

	// Check for its:dir attribute
	dirAttr := getAttr(elem.Attr, itsNS, "dir")

	// Read the text content
	var textContent strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, nil, fmt.Errorf("error reading property content: %w", err)
		}

		switch t := token.(type) {
		case xml.CharData:
			textContent.Write(t)
		case xml.EndElement:
			// End of property element
			var object Term
			if datatypeAttr != "" {
				// Typed literal
				object = &Literal{
					Value:    textContent.String(),
					Datatype: NewNamedNode(datatypeAttr),
				}
			} else if langAttr != "" {
				// Language-tagged literal with optional direction
				object = &Literal{
					Value:     textContent.String(),
					Language:  langAttr,
					Direction: dirAttr,
				}
			} else {
				// Plain literal
				object = NewLiteral(textContent.String())
			}
			return object, nil, nil

		case xml.StartElement:
			// Nested element (blank node or another Description)
			if t.Name.Local == "Description" && t.Name.Space == rdfNS {
				// Check for rdf:about, rdf:ID, or rdf:nodeID
				aboutAttr := getAttr(t.Attr, rdfNS, "about")
				idAttr := getAttr(t.Attr, rdfNS, "ID")
				nodeIDAttr := getAttr(t.Attr, rdfNS, "nodeID")

				var subject Term
				if aboutAttr != "" {
					subject = NewNamedNode(p.resolveURI(aboutAttr))
				} else if idAttr != "" {
					resolvedID, err := p.resolveID(idAttr)
					if err != nil {
						return nil, nil, err
					}
					subject = NewNamedNode(resolvedID)
				} else if nodeIDAttr != "" {
					node, err := p.getOrCreateNodeID(nodeIDAttr, blankNodeCounter)
					if err != nil {
						return nil, nil, err
					}
					subject = node
				} else {
					// Nested blank node without attributes
					*blankNodeCounter++
					subject = NewBlankNode(fmt.Sprintf("b%d", *blankNodeCounter))
				}

				return subject, nil, nil
			}
		}
	}
}

// parseNestedDescription parses a nested rdf:Description element
func (p *RDFXMLParser) parseNestedDescription(decoder *xml.Decoder, subject Term, blankNodeCounter *int) ([]*Quad, error) {
	var quads []*Quad
	var liCounter int // Each Description has its own rdf:li counter

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("error parsing nested description: %w", err)
		}

		switch elem := token.(type) {
		case xml.StartElement:
			// Validate attributes for illegal combinations
			if err := validateAttributes(elem); err != nil {
				return nil, err
			}

			// Handle rdf:li specially - it auto-numbers to rdf:_N
			var predicate string
			if elem.Name.Local == "li" && elem.Name.Space == rdfNS {
				liCounter++
				predicate = fmt.Sprintf("%s_%d", rdfNS, liCounter)
			} else {
				predicate = elem.Name.Space + elem.Name.Local
			}

			// Capture rdf:ID early for reification
			propertyIDAttr := getAttr(elem.Attr, rdfNS, "ID")

			// Check for rdf:resource attribute
			resourceAttr := getAttr(elem.Attr, rdfNS, "resource")
			if resourceAttr != "" {
				object := NewNamedNode(p.resolveURI(resourceAttr))
				quad := NewQuad(subject, NewNamedNode(predicate), object, NewDefaultGraph())
				quads = append(quads, quad)

				// Check for rdf:ID on property element (triggers reification)
				if idAttr := getAttr(elem.Attr, rdfNS, "ID"); idAttr != "" {
					statementID, err := p.resolveID(idAttr)
					if err != nil {
						return nil, fmt.Errorf("invalid RDF/XML: %w", err)
					}
					reificationQuads := generateReificationQuads(statementID, subject, NewNamedNode(predicate), object)
					quads = append(quads, reificationQuads...)
				}

				continue
			}

			// Check for rdf:nodeID attribute (blank node object)
			nodeIDAttr := getAttr(elem.Attr, rdfNS, "nodeID")
			if nodeIDAttr != "" {
				object, err := p.getOrCreateNodeID(nodeIDAttr, blankNodeCounter)
				if err != nil {
					return nil, fmt.Errorf("invalid RDF/XML: %w", err)
				}
				quad := NewQuad(subject, NewNamedNode(predicate), object, NewDefaultGraph())
				quads = append(quads, quad)

				// Check for rdf:ID on property element (triggers reification)
				if propertyIDAttr != "" {
					statementID, err := p.resolveID(propertyIDAttr)
					if err != nil {
						return nil, fmt.Errorf("invalid RDF/XML: %w", err)
					}
					reificationQuads := generateReificationQuads(statementID, subject, NewNamedNode(predicate), object)
					quads = append(quads, reificationQuads...)
				}

				continue
			}

			// Read text content
			var textContent strings.Builder
			done := false
			for !done {
				token, err := decoder.Token()
				if err != nil {
					return nil, err
				}

				switch t := token.(type) {
				case xml.CharData:
					textContent.Write(t)
				case xml.EndElement:
					object := NewLiteral(textContent.String())
					quad := NewQuad(subject, NewNamedNode(predicate), object, NewDefaultGraph())
					quads = append(quads, quad)

					// Check for rdf:ID on property element (triggers reification)
					if propertyIDAttr != "" {
						statementID, err := p.resolveID(propertyIDAttr)
						if err != nil {
							return nil, fmt.Errorf("invalid RDF/XML: %w", err)
						}
						reificationQuads := generateReificationQuads(statementID, subject, NewNamedNode(predicate), object)
						quads = append(quads, reificationQuads...)
					}
					done = true
				}
			}

		case xml.EndElement:
			if elem.Name.Local == "Description" && elem.Name.Space == rdfNS {
				return quads, nil
			}
		}
	}
}

// getAttr gets an attribute value by namespace and local name
func getAttr(attrs []xml.Attr, namespace, local string) string {
	for _, attr := range attrs {
		if attr.Name.Space == namespace && attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

// getAttrAny gets an attribute value by local name (any namespace)
func getAttrAny(attrs []xml.Attr, local string) string {
	for _, attr := range attrs {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}
