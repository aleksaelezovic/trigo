package rdf

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// RDFXMLParser parses RDF/XML format
// Note: This is a simplified parser that handles common RDF/XML patterns.
// It supports:
// - rdf:Description elements
// - Properties as XML elements
// - rdf:about, rdf:resource attributes
// - rdf:datatype, xml:lang attributes
// - Nested blank nodes
//
// Not yet supported:
// - rdf:parseType="Resource"
// - rdf:parseType="Collection"
// - Property attributes on Description elements
// - RDF containers (rdf:Bag, rdf:Seq, rdf:Alt)
type RDFXMLParser struct{}

// NewRDFXMLParser creates a new RDF/XML parser
func NewRDFXMLParser() *RDFXMLParser {
	return &RDFXMLParser{}
}

const (
	rdfNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
)

// Parse parses RDF/XML and returns quads (all in default graph)
func (p *RDFXMLParser) Parse(reader io.Reader) ([]*Quad, error) {
	decoder := xml.NewDecoder(reader)
	var quads []*Quad
	var currentSubject Term
	var blankNodeCounter int

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
			// Check if this is rdf:RDF (root element) - skip it
			if elem.Name.Local == "RDF" && elem.Name.Space == rdfNS {
				continue
			}

			// Check if this is rdf:Description (subject)
			if elem.Name.Local == "Description" && elem.Name.Space == rdfNS {
				// Get rdf:about attribute for subject
				aboutAttr := getAttr(elem.Attr, rdfNS, "about")
				if aboutAttr != "" {
					currentSubject = NewNamedNode(aboutAttr)
				} else {
					// Blank node
					blankNodeCounter++
					currentSubject = NewBlankNode(fmt.Sprintf("b%d", blankNodeCounter))
				}
				continue
			}

			// This is a property element
			if currentSubject != nil {
				predicate := elem.Name.Space + elem.Name.Local

				// Check for rdf:resource attribute (object is IRI)
				resourceAttr := getAttr(elem.Attr, rdfNS, "resource")
				if resourceAttr != "" {
					object := NewNamedNode(resourceAttr)
					quad := NewQuad(currentSubject, NewNamedNode(predicate), object, NewDefaultGraph())
					quads = append(quads, quad)
					continue
				}

				// Check for rdf:datatype attribute
				datatypeAttr := getAttr(elem.Attr, rdfNS, "datatype")

				// Check for xml:lang attribute
				langAttr := getAttrAny(elem.Attr, "lang")

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
							// Language-tagged literal
							object = &Literal{
								Value:    textContent.String(),
								Language: langAttr,
							}
						} else {
							// Plain literal
							object = NewLiteral(textContent.String())
						}

						quad := NewQuad(currentSubject, NewNamedNode(predicate), object, NewDefaultGraph())
						quads = append(quads, quad)
						goto propertyDone
					case xml.StartElement:
						// Nested element (blank node or another Description)
						if t.Name.Local == "Description" && t.Name.Space == rdfNS {
							// Nested blank node
							blankNodeCounter++
							object := NewBlankNode(fmt.Sprintf("b%d", blankNodeCounter))
							quad := NewQuad(currentSubject, NewNamedNode(predicate), object, NewDefaultGraph())
							quads = append(quads, quad)

							// Parse nested Description
							nestedQuads, err := p.parseNestedDescription(decoder, object, &blankNodeCounter)
							if err != nil {
								return nil, err
							}
							quads = append(quads, nestedQuads...)
							goto propertyDone
						}
					}
				}
			propertyDone:
			}

		case xml.EndElement:
			// End of rdf:Description - reset current subject
			if elem.Name.Local == "Description" && elem.Name.Space == rdfNS {
				currentSubject = nil
			}
		}
	}

	return quads, nil
}

// parseNestedDescription parses a nested rdf:Description element
func (p *RDFXMLParser) parseNestedDescription(decoder *xml.Decoder, subject Term, blankNodeCounter *int) ([]*Quad, error) {
	var quads []*Quad

	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("error parsing nested description: %w", err)
		}

		switch elem := token.(type) {
		case xml.StartElement:
			// Property element
			predicate := elem.Name.Space + elem.Name.Local

			// Check for rdf:resource attribute
			resourceAttr := getAttr(elem.Attr, rdfNS, "resource")
			if resourceAttr != "" {
				object := NewNamedNode(resourceAttr)
				quad := NewQuad(subject, NewNamedNode(predicate), object, NewDefaultGraph())
				quads = append(quads, quad)
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
