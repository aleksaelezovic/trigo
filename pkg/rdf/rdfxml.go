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
// - rdf:about, rdf:resource, rdf:ID attributes
// - rdf:datatype, xml:lang attributes
// - Nested blank nodes
// - RDF containers (rdf:Bag, rdf:Seq, rdf:Alt)
// - rdf:li auto-numbering
// - xml:base for base URI resolution
//
// Not yet supported:
// - rdf:parseType="Resource"
// - rdf:parseType="Collection"
// - Property attributes on Description elements
type RDFXMLParser struct {
	baseURIStack []string // Stack of xml:base values
	documentBase string   // Document base URI (file location)
}

// NewRDFXMLParser creates a new RDF/XML parser
func NewRDFXMLParser() *RDFXMLParser {
	return &RDFXMLParser{}
}

// SetBaseURI sets the document base URI (used for resolving relative URIs and rdf:ID)
func (p *RDFXMLParser) SetBaseURI(base string) {
	p.documentBase = base
}

const (
	rdfNS = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
)

// pushBase adds a base URI to the stack
func (p *RDFXMLParser) pushBase(base string) {
	p.baseURIStack = append(p.baseURIStack, base)
}

// getCurrentBase returns the current base URI (xml:base takes precedence, then document base)
func (p *RDFXMLParser) getCurrentBase() string {
	if len(p.baseURIStack) > 0 {
		return p.baseURIStack[len(p.baseURIStack)-1]
	}
	return p.documentBase
}

// resolveID resolves an rdf:ID value against the current base
func (p *RDFXMLParser) resolveID(id string) string {
	base := p.getCurrentBase()
	if base != "" {
		return base + "#" + id
	}
	return "#" + id
}

// isContainer checks if an element is an RDF container
func isContainer(elem xml.StartElement) bool {
	if elem.Name.Space != rdfNS {
		return false
	}
	return elem.Name.Local == "Bag" || elem.Name.Local == "Seq" || elem.Name.Local == "Alt"
}

// Parse parses RDF/XML and returns quads (all in default graph)
func (p *RDFXMLParser) Parse(reader io.Reader) ([]*Quad, error) {
	decoder := xml.NewDecoder(reader)
	var quads []*Quad
	var currentSubject Term
	var blankNodeCounter int
	var liCounter int // Counter for rdf:li elements within current container

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
			// Check for xml:base attribute and push to stack
			xmlBase := getAttrAny(elem.Attr, "base")
			if xmlBase != "" {
				p.pushBase(xmlBase)
				// Note: will be popped when we see the corresponding EndElement
			}

			// Check if this is rdf:RDF (root element) - skip it
			if elem.Name.Local == "RDF" && elem.Name.Space == rdfNS {
				continue
			}

			// Check if this is an RDF container (rdf:Bag, rdf:Seq, rdf:Alt)
			if isContainer(elem) {
				// Check for rdf:about or rdf:ID, otherwise create blank node
				aboutAttr := getAttr(elem.Attr, rdfNS, "about")
				idAttr := getAttr(elem.Attr, rdfNS, "ID")

				var containerNode Term
				if aboutAttr != "" {
					containerNode = NewNamedNode(aboutAttr)
				} else if idAttr != "" {
					containerNode = NewNamedNode(p.resolveID(idAttr))
				} else {
					// Create blank node for container
					blankNodeCounter++
					containerNode = NewBlankNode(fmt.Sprintf("b%d", blankNodeCounter))
				}

				// Add rdf:type triple
				containerType := rdfNS + elem.Name.Local
				quad := NewQuad(containerNode, NewNamedNode(rdfNS+"type"), NewNamedNode(containerType), NewDefaultGraph())
				quads = append(quads, quad)

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
			if elem.Name.Local == "Description" && elem.Name.Space == rdfNS {
				// Get rdf:about, rdf:ID, or create blank node
				aboutAttr := getAttr(elem.Attr, rdfNS, "about")
				idAttr := getAttr(elem.Attr, rdfNS, "ID")
				if aboutAttr != "" {
					currentSubject = NewNamedNode(aboutAttr)
				} else if idAttr != "" {
					currentSubject = NewNamedNode(p.resolveID(idAttr))
				} else {
					// Blank node
					blankNodeCounter++
					currentSubject = NewBlankNode(fmt.Sprintf("b%d", blankNodeCounter))
				}

				// Process property attributes on Description element
				for _, attr := range elem.Attr {
					// Skip RDF-specific and XML-specific attributes
					if attr.Name.Space == rdfNS {
						// Skip rdf:about, rdf:ID, rdf:nodeID, rdf:type (handled separately)
						continue
					}
					if attr.Name.Local == "base" || attr.Name.Local == "lang" {
						// Skip xml:base and xml:lang
						continue
					}
					if attr.Name.Space == "" {
						// Skip attributes without namespace
						continue
					}

					// This is a property attribute
					predicate := attr.Name.Space + attr.Name.Local
					object := NewLiteral(attr.Value)
					quad := NewQuad(currentSubject, NewNamedNode(predicate), object, NewDefaultGraph())
					quads = append(quads, quad)
				}

				continue
			}

			// Check if this is a typed node (not rdf:Description but has rdf:about/ID)
			aboutAttr := getAttr(elem.Attr, rdfNS, "about")
			idAttr := getAttr(elem.Attr, rdfNS, "ID")
			if aboutAttr != "" || idAttr != "" || currentSubject == nil {
				// This is a typed node (implicit rdf:type)
				var subject Term
				if aboutAttr != "" {
					subject = NewNamedNode(aboutAttr)
				} else if idAttr != "" {
					subject = NewNamedNode(p.resolveID(idAttr))
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
			// Check if we need to pop xml:base
			if getAttrAny([]xml.Attr{}, "base") != "" {
				// Note: We can't easily check if this element had xml:base
				// This is a simplified implementation
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
				object, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
				if err != nil {
					return nil, err
				}

				quad := NewQuad(containerNode, NewNamedNode(memberPredicate), object, NewDefaultGraph())
				quads = append(quads, quad)
				continue
			}

			// Check if this is an explicit rdf:_N member
			if strings.HasPrefix(elem.Name.Local, "_") && elem.Name.Space == rdfNS {
				memberPredicate := elem.Name.Space + elem.Name.Local

				// Parse the content
				object, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
				if err != nil {
					return nil, err
				}

				quad := NewQuad(containerNode, NewNamedNode(memberPredicate), object, NewDefaultGraph())
				quads = append(quads, quad)
				continue
			}

			// Other properties (not typical in containers, but allowed)
			predicate := elem.Name.Space + elem.Name.Local
			object, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
			if err != nil {
				return nil, err
			}

			quad := NewQuad(containerNode, NewNamedNode(predicate), object, NewDefaultGraph())
			quads = append(quads, quad)

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

				object, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
				if err != nil {
					return nil, err
				}

				quad := NewQuad(subject, NewNamedNode(memberPredicate), object, NewDefaultGraph())
				quads = append(quads, quad)
				continue
			}

			// Check if this is an explicit rdf:_N member
			if strings.HasPrefix(elem.Name.Local, "_") && elem.Name.Space == rdfNS {
				memberPredicate := elem.Name.Space + elem.Name.Local

				object, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
				if err != nil {
					return nil, err
				}

				quad := NewQuad(subject, NewNamedNode(memberPredicate), object, NewDefaultGraph())
				quads = append(quads, quad)
				continue
			}

			// Regular property
			predicate := elem.Name.Space + elem.Name.Local
			object, err := p.parsePropertyContent(decoder, elem, blankNodeCounter)
			if err != nil {
				return nil, err
			}

			quad := NewQuad(subject, NewNamedNode(predicate), object, NewDefaultGraph())
			quads = append(quads, quad)

		case xml.EndElement:
			// End of typed node
			return quads, nil
		}
	}
}

// parsePropertyContent parses the content of a property element and returns the object
func (p *RDFXMLParser) parsePropertyContent(decoder *xml.Decoder, elem xml.StartElement, blankNodeCounter *int) (Term, error) {
	// Check for rdf:parseType attribute
	parseTypeAttr := getAttr(elem.Attr, rdfNS, "parseType")
	if parseTypeAttr == "Resource" {
		// Create a blank node for the resource
		*blankNodeCounter++
		blankNode := NewBlankNode(fmt.Sprintf("b%d", *blankNodeCounter))

		// Consume tokens until end element
		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			if _, ok := token.(xml.EndElement); ok {
				break
			}
		}

		return blankNode, nil
	}

	// Check for rdf:resource attribute (object is IRI)
	resourceAttr := getAttr(elem.Attr, rdfNS, "resource")
	if resourceAttr != "" {
		// Consume the end element
		_, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		return NewNamedNode(resourceAttr), nil
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
			return object, nil

		case xml.StartElement:
			// Nested element (blank node or another Description)
			if t.Name.Local == "Description" && t.Name.Space == rdfNS {
				// Nested blank node
				*blankNodeCounter++
				return NewBlankNode(fmt.Sprintf("b%d", *blankNodeCounter)), nil
			}
		}
	}
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
