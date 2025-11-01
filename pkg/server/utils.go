package server

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aleksaelezovic/trigo/pkg/server/results"
	"github.com/aleksaelezovic/trigo/pkg/sparql/executor"
)

// writeError writes an error response
func (s *Server) writeError(w http.ResponseWriter, statusCode int, message string) {
	log.Printf("Error: %s", message)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	// Simple JSON serialization
	_, _ = w.Write([]byte(fmt.Sprintf(`{"error":{"code":%d,"message":"%s"}}`, statusCode, message))) // #nosec G104 - error writing response is logged elsewhere if needed
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
	if strings.Contains(accept, "text/csv") {
		return "csv"
	}
	if strings.Contains(accept, "text/tab-separated-values") {
		return "tsv"
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
		data, err = results.FormatConstructResultNTriples(constructResult)
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
			data, err = results.FormatSelectResultsXML(selectResult)
		} else if askResult, ok := result.(*executor.AskResult); ok {
			data, err = results.FormatAskResultXML(askResult)
		}

	case "csv":
		contentType = "text/csv; charset=utf-8"

		if selectResult, ok := result.(*executor.SelectResult); ok {
			data, err = results.FormatSelectResultsCSV(selectResult)
		} else if askResult, ok := result.(*executor.AskResult); ok {
			data, err = results.FormatAskResultCSV(askResult)
		}

	case "tsv":
		contentType = "text/tab-separated-values; charset=utf-8"

		if selectResult, ok := result.(*executor.SelectResult); ok {
			data, err = results.FormatSelectResultsTSV(selectResult)
		} else if askResult, ok := result.(*executor.AskResult); ok {
			data, err = results.FormatAskResultTSV(askResult)
		}

	default: // json
		contentType = "application/sparql-results+json; charset=utf-8"

		if selectResult, ok := result.(*executor.SelectResult); ok {
			data, err = results.FormatSelectResultsJSON(selectResult)
		} else if askResult, ok := result.(*executor.AskResult); ok {
			data, err = results.FormatAskResultJSON(askResult)
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
