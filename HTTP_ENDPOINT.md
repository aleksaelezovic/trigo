# HTTP SPARQL Endpoint

Trigo provides a standard W3C SPARQL 1.1 Protocol compliant HTTP endpoint.

## Starting the Server

```bash
# Build the application
go build -o trigo ./cmd/trigo

# Start with default address (localhost:8080)
./trigo serve

# Start with custom address
./trigo serve localhost:3000
./trigo serve 0.0.0.0:8080
```

## Endpoint URL

```
http://localhost:8080/sparql
```

## Protocol Support

Trigo implements the [W3C SPARQL 1.1 Protocol](https://www.w3.org/TR/sparql11-protocol/) specification.

### HTTP Methods

#### GET Request

Query string passed via `query` URL parameter:

```bash
curl -G http://localhost:8080/sparql \
  --data-urlencode 'query=SELECT ?s ?p ?o WHERE { ?s ?p ?o } LIMIT 10' \
  -H 'Accept: application/sparql-results+json'
```

#### POST Request

Three content-type options:

**1. Direct SPARQL Query (Recommended)**

```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/sparql-query' \
  -H 'Accept: application/sparql-results+json' \
  -d 'SELECT ?s ?p ?o WHERE { ?s ?p ?o } LIMIT 10'
```

**2. Form-encoded**

```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -H 'Accept: application/sparql-results+json' \
  --data-urlencode 'query=SELECT ?s ?p ?o WHERE { ?s ?p ?o } LIMIT 10'
```

**3. Query in Body (fallback)**

```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Accept: application/sparql-results+json' \
  -d 'SELECT ?s ?p ?o WHERE { ?s ?p ?o } LIMIT 10'
```

## Response Formats

Trigo supports content negotiation via the `Accept` header:

### SPARQL JSON Results (Default)

**Spec:** https://www.w3.org/TR/sparql11-results-json/

```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/sparql-query' \
  -H 'Accept: application/sparql-results+json' \
  -d 'SELECT ?person ?name WHERE {
        ?person <http://xmlns.com/foaf/0.1/name> ?name
      }'
```

**Response:**
```json
{
  "head": {
    "vars": ["person", "name"]
  },
  "results": {
    "bindings": [
      {
        "person": {
          "type": "uri",
          "value": "http://example.org/alice"
        },
        "name": {
          "type": "literal",
          "value": "Alice"
        }
      }
    ]
  }
}
```

### SPARQL XML Results

**Spec:** https://www.w3.org/TR/rdf-sparql-XMLres/

```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/sparql-query' \
  -H 'Accept: application/sparql-results+xml' \
  -d 'SELECT ?s ?p ?o WHERE { ?s ?p ?o } LIMIT 1'
```

**Response:**
```xml
<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
  <head>
    <variable name="s"/>
    <variable name="p"/>
    <variable name="o"/>
  </head>
  <results>
    <result>
      <binding name="s">
        <uri>http://example.org/alice</uri>
      </binding>
      <binding name="p">
        <uri>http://xmlns.com/foaf/0.1/name</uri>
      </binding>
      <binding name="o">
        <literal>Alice</literal>
      </binding>
    </result>
  </results>
</sparql>
```

## Query Types

### SELECT Queries

```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/sparql-query' \
  -H 'Accept: application/sparql-results+json' \
  -d 'SELECT ?person ?name ?age WHERE {
        ?person <http://xmlns.com/foaf/0.1/name> ?name .
        ?person <http://xmlns.com/foaf/0.1/age> ?age .
      }
      ORDER BY ?name
      LIMIT 10'
```

### ASK Queries

```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/sparql-query' \
  -H 'Accept: application/sparql-results+json' \
  -d 'ASK WHERE {
        <http://example.org/alice> <http://xmlns.com/foaf/0.1/name> "Alice"
      }'
```

**Response:**
```json
{
  "head": {
    "vars": []
  },
  "boolean": true
}
```

## Web Interface

Visit `http://localhost:8080/` in your browser for a full-featured SPARQL query interface powered by [YASGUI](https://github.com/zazuko/Yasgui).

### Features

- **Interactive Query Editor**
  - Syntax highlighting for SPARQL queries
  - Auto-completion for keywords and IRIs
  - Multi-line editing with proper indentation

- **Result Visualization**
  - Table view with sortable columns
  - Pivot tables for data analysis
  - Chart visualizations (bar, line, pie)
  - Raw JSON/XML response view

- **Query Management**
  - Save queries for reuse
  - Query history with timestamps
  - Multiple query tabs
  - Share queries via URL

- **Endpoint Information**
  - Display current endpoint URL
  - Show database statistics (triple count)
  - Real-time query execution

### Quick Start with Web UI

1. Start the server:
   ```bash
   ./trigo serve
   ```

2. Open browser to `http://localhost:8080/`

3. Enter a SPARQL query in the editor:
   ```sparql
   SELECT ?s ?p ?o
   WHERE {
     ?s ?p ?o .
   }
   LIMIT 10
   ```

4. Click "Execute" or press Ctrl+Enter

5. View results in table, chart, or raw format

### YASGUI Configuration

The UI is pre-configured to use the local `/sparql` endpoint. No additional setup required!

## CORS Support

The endpoint includes CORS headers, allowing queries from web applications:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, OPTIONS
Access-Control-Allow-Headers: Content-Type, Accept
```

## Error Responses

Errors are returned as JSON with appropriate HTTP status codes:

```json
{
  "error": {
    "code": 400,
    "message": "Parse error: expected WHERE clause"
  }
}
```

### Common Status Codes

- `200 OK` - Query executed successfully
- `400 Bad Request` - Invalid query syntax or missing parameters
- `405 Method Not Allowed` - Unsupported HTTP method
- `500 Internal Server Error` - Query execution or optimization error

## Client Examples

### Python (requests)

```python
import requests

query = """
SELECT ?person ?name
WHERE {
    ?person <http://xmlns.com/foaf/0.1/name> ?name .
}
LIMIT 5
"""

response = requests.post(
    'http://localhost:8080/sparql',
    data=query,
    headers={
        'Content-Type': 'application/sparql-query',
        'Accept': 'application/sparql-results+json'
    }
)

results = response.json()
for binding in results['results']['bindings']:
    person = binding['person']['value']
    name = binding['name']['value']
    print(f"{person} -> {name}")
```

### JavaScript (fetch)

```javascript
const query = `
  SELECT ?person ?name
  WHERE {
    ?person <http://xmlns.com/foaf/0.1/name> ?name .
  }
  LIMIT 5
`;

fetch('http://localhost:8080/sparql', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/sparql-query',
    'Accept': 'application/sparql-results+json'
  },
  body: query
})
.then(response => response.json())
.then(data => {
  data.results.bindings.forEach(binding => {
    console.log(`${binding.person.value} -> ${binding.name.value}`);
  });
});
```

### cURL Examples

**Find all triples:**
```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/sparql-query' \
  -d 'SELECT * WHERE { ?s ?p ?o } LIMIT 100'
```

**Filter by predicate:**
```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/sparql-query' \
  -d 'SELECT ?s ?o WHERE {
        ?s <http://xmlns.com/foaf/0.1/knows> ?o
      }'
```

**Multiple patterns:**
```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/sparql-query' \
  -d 'SELECT ?person ?friend ?friendName WHERE {
        ?person <http://xmlns.com/foaf/0.1/knows> ?friend .
        ?friend <http://xmlns.com/foaf/0.1/name> ?friendName .
      }'
```

**DISTINCT results:**
```bash
curl -X POST http://localhost:8080/sparql \
  -H 'Content-Type: application/sparql-query' \
  -d 'SELECT DISTINCT ?predicate WHERE {
        ?s ?predicate ?o
      }'
```

## Performance Tips

1. **Use LIMIT**: Always add `LIMIT` for exploratory queries
2. **Bind Variables**: More bound terms = faster queries
3. **Index Selection**: The server automatically selects the optimal index
4. **Connection Pooling**: Reuse HTTP connections for multiple queries

## Integration with Other Tools

### Apache Jena (Java)

```java
import org.apache.jena.query.*;

String sparqlEndpoint = "http://localhost:8080/sparql";
String queryString = "SELECT ?s ?p ?o WHERE { ?s ?p ?o } LIMIT 10";

QueryExecution qexec = QueryExecutionFactory.sparqlService(
    sparqlEndpoint,
    queryString
);

ResultSet results = qexec.execSelect();
ResultSetFormatter.out(System.out, results);
qexec.close();
```

### RDFLib (Python)

```python
from SPARQLWrapper import SPARQLWrapper, JSON

sparql = SPARQLWrapper("http://localhost:8080/sparql")
sparql.setQuery("""
    SELECT ?person ?name
    WHERE {
        ?person <http://xmlns.com/foaf/0.1/name> ?name .
    }
""")
sparql.setReturnFormat(JSON)

results = sparql.query().convert()
for result in results["results"]["bindings"]:
    print(f"{result['person']['value']} -> {result['name']['value']}")
```

## Security Considerations

### Current Implementation

- No authentication/authorization
- Open to all clients (CORS: *)
- Intended for development and trusted environments

### Production Deployment

For production use, consider adding:
- Authentication (API keys, OAuth)
- Rate limiting
- Query complexity limits
- Timeouts
- Network-level access control (firewall, VPN)
- Reverse proxy (nginx, Apache) with:
  - HTTPS/TLS
  - Request filtering
  - Caching
  - Load balancing

## Next Steps

- Read [README.md](README.md) for overview
- Read [ARCHITECTURE.md](ARCHITECTURE.md) for technical details
- Read [QUICKSTART.md](QUICKSTART.md) for getting started
