package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aleksaelezovic/trigo/internal/rdfio"
	"github.com/aleksaelezovic/trigo/internal/sparql/executor"
	"github.com/aleksaelezovic/trigo/internal/sparql/optimizer"
	"github.com/aleksaelezovic/trigo/internal/sparql/parser"
	"github.com/aleksaelezovic/trigo/internal/store"
)

// Server represents the HTTP SPARQL server
type Server struct {
	store    *store.TripleStore
	executor *executor.Executor
	optimizer *optimizer.Optimizer
	addr     string
}

// NewServer creates a new SPARQL HTTP server
func NewServer(store *store.TripleStore, addr string) *Server {
	exec := executor.NewExecutor(store)

	// Get statistics for optimizer
	count, _ := store.Count()
	stats := &optimizer.Statistics{TotalTriples: count}
	opt := optimizer.NewOptimizer(stats)

	return &Server{
		store:    store,
		executor: exec,
		optimizer: opt,
		addr:     addr,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/sparql", s.handleSPARQL)
	mux.HandleFunc("/data", s.handleDataUpload)
	mux.HandleFunc("/", s.handleRoot)

	server := &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting SPARQL endpoint at http://%s/sparql", s.addr)
	return server.ListenAndServe()
}

// handleRoot provides information about the endpoint
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Get current endpoint URL from request
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	endpointURL := fmt.Sprintf("%s://%s/sparql", scheme, r.Host)

	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Trigo SPARQL Endpoint</title>
    <link href="https://unpkg.com/@zazuko/yasgui@4.5.0/build/yasgui.min.css" rel="stylesheet" type="text/css" />
    <script src="https://unpkg.com/@zazuko/yasgui@4.5.0/build/yasgui.min.js"></script>
    <style>
        body {
            margin: 0;
            padding: 0;
            font-family: Arial, sans-serif;
            display: flex;
            flex-direction: column;
            height: 100vh;
        }
        .header {
            background: #2c3e50;
            color: white;
            padding: 15px 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header h1 {
            margin: 0;
            font-size: 24px;
            font-weight: 500;
        }
        .header .info {
            margin-top: 5px;
            font-size: 14px;
            opacity: 0.9;
        }
        .header .info code {
            background: rgba(255,255,255,0.2);
            padding: 2px 6px;
            border-radius: 3px;
            font-family: monospace;
        }
        #yasgui {
            flex: 1;
            overflow: hidden;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🎯 Trigo SPARQL Endpoint</h1>
        <div class="info">
            Endpoint: <code>` + endpointURL + `</code> |
            Total triples: <strong>` + fmt.Sprintf("%d", s.Stats().TotalTriples) + `</strong>
        </div>
    </div>
    <div id="yasgui"></div>
    <script>
        const yasgui = new Yasgui(document.getElementById("yasgui"), {
            requestConfig: {
                endpoint: "` + endpointURL + `",
                method: "POST"
            },
            copyEndpointOnNewTab: false,
            endpointCatalogueOptions: {
                getData: function() {
                    return [
                        {
                            endpoint: "` + endpointURL + `",
                            label: "Trigo Local"
                        }
                    ];
                }
            }
        });
    </script>
</body>
</html>`

	_, _ = w.Write([]byte(html)) // #nosec G104 - error writing response is logged elsewhere if needed
}

// handleSPARQL handles SPARQL query requests according to SPARQL 1.1 Protocol
// https://www.w3.org/TR/sparql11-protocol/
func (s *Server) handleSPARQL(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Extract query string
	var queryString string
	var err error

	switch r.Method {
	case "GET":
		// GET request: query in URL parameter
		queryString = r.URL.Query().Get("query")
		if queryString == "" {
			s.writeError(w, http.StatusBadRequest, "Missing 'query' parameter")
			return
		}

	case "POST":
		// POST request: query in body
		contentType := r.Header.Get("Content-Type")

		if strings.Contains(contentType, "application/sparql-query") {
			// Direct SPARQL query in body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				s.writeError(w, http.StatusBadRequest, "Failed to read request body")
				return
			}
			queryString = string(body)

		} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			// Form-encoded: query parameter
			if err := r.ParseForm(); err != nil {
				s.writeError(w, http.StatusBadRequest, "Failed to parse form")
				return
			}
			queryString = r.FormValue("query")
			if queryString == "" {
				s.writeError(w, http.StatusBadRequest, "Missing 'query' parameter")
				return
			}

		} else {
			// Try to read body as query string anyway
			body, err := io.ReadAll(r.Body)
			if err != nil {
				s.writeError(w, http.StatusBadRequest, "Failed to read request body")
				return
			}
			queryString = string(body)
		}

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed. Use GET or POST")
		return
	}

	if queryString == "" {
		s.writeError(w, http.StatusBadRequest, "Empty query")
		return
	}

	// Parse query
	p := parser.NewParser(queryString)
	query, err := p.Parse()
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Parse error: %v", err))
		return
	}

	// Optimize query
	optimizedQuery, err := s.optimizer.Optimize(query)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Optimization error: %v", err))
		return
	}

	// Execute query
	result, err := s.executor.Execute(optimizedQuery)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Execution error: %v", err))
		return
	}

	// Determine response format based on Accept header
	acceptHeader := r.Header.Get("Accept")
	format := s.negotiateFormat(acceptHeader)

	// Format and send response
	s.writeResult(w, result, format)
}

// Stats returns the optimizer statistics
func (s *Server) Stats() *optimizer.Statistics {
	// Update statistics
	count, _ := s.store.Count()
	return &optimizer.Statistics{TotalTriples: count}
}

// negotiateFormat determines the response format based on Accept header
func (s *Server) negotiateFormat(acceptHeader string) string {
	accept := strings.ToLower(acceptHeader)

	// Check for specific format requests
	if strings.Contains(accept, "application/sparql-results+xml") {
		return "xml"
	}
	if strings.Contains(accept, "application/sparql-results+json") {
		return "json"
	}
	if strings.Contains(accept, "application/json") {
		return "json"
	}
	if strings.Contains(accept, "text/xml") || strings.Contains(accept, "application/xml") {
		return "xml"
	}

	// Default to JSON
	return "json"
}

// writeResult writes the query result in the specified format
func (s *Server) writeResult(w http.ResponseWriter, result executor.QueryResult, format string) {
	var data []byte
	var err error
	var contentType string

	// Handle CONSTRUCT results separately (they return RDF, not SPARQL results)
	if constructResult, ok := result.(*executor.ConstructResult); ok {
		// CONSTRUCT queries return RDF triples in N-Triples format
		contentType = "application/n-triples; charset=utf-8"
		data, err = FormatConstructResultNTriples(constructResult)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Formatting error: %v", err))
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data) // #nosec G104 - error writing response is logged elsewhere if needed
		return
	}

	// Handle SELECT and ASK results
	switch format {
	case "xml":
		contentType = "application/sparql-results+xml; charset=utf-8"

		if selectResult, ok := result.(*executor.SelectResult); ok {
			data, err = FormatSelectResultsXML(selectResult)
		} else if askResult, ok := result.(*executor.AskResult); ok {
			data, err = FormatAskResultXML(askResult)
		}

	default: // json
		contentType = "application/sparql-results+json; charset=utf-8"

		if selectResult, ok := result.(*executor.SelectResult); ok {
			data, err = FormatSelectResultsJSON(selectResult)
		} else if askResult, ok := result.(*executor.AskResult); ok {
			data, err = FormatAskResultJSON(askResult)
		}
	}

	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Formatting error: %v", err))
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data) // #nosec G104 - error writing response is logged elsewhere if needed
}

// handleDataUpload handles bulk data uploads in various RDF formats
func (s *Server) handleDataUpload(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed. Use POST")
		return
	}

	// Get Content-Type header
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		s.writeError(w, http.StatusBadRequest, "Missing Content-Type header")
		return
	}

	// Create appropriate parser based on content type
	parser, err := rdfio.NewParser(contentType)
	if err != nil {
		supportedTypes := rdfio.GetSupportedContentTypes()
		s.writeError(w, http.StatusUnsupportedMediaType,
			fmt.Sprintf("Unsupported content type: %s. Supported types: %v", contentType, supportedTypes))
		return
	}

	// Parse RDF data from request body
	startTime := time.Now()
	quads, err := parser.Parse(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Parse error: %v", err))
		return
	}

	// Bulk insert quads
	if err := s.store.InsertQuadsBatch(quads); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Insert error: %v", err))
		return
	}

	duration := time.Since(startTime)

	// Return success response with statistics
	response := map[string]interface{}{
		"success": true,
		"statistics": map[string]interface{}{
			"quadsInserted":     len(quads),
			"durationMs":        duration.Milliseconds(),
			"quadsPerSecond":    float64(len(quads)) / duration.Seconds(),
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response) // #nosec G104 - error writing response is logged elsewhere if needed
}

// writeError writes an error response
func (s *Server) writeError(w http.ResponseWriter, statusCode int, message string) {
	log.Printf("Error: %s", message)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	// Simple JSON serialization
	_, _ = w.Write([]byte(fmt.Sprintf(`{"error":{"code":%d,"message":"%s"}}`, statusCode, message))) // #nosec G104 - error writing response is logged elsewhere if needed
}
