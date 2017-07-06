package render

import (
	"fmt"
	"strings"

	"github.com/osteele/liquid/expression"
)

// A ParseError is a parse error during the template parsing.
type ParseError string

func (e ParseError) Error() string { return string(e) }

func parseErrorf(format string, a ...interface{}) ParseError {
	return ParseError(fmt.Sprintf(format, a...))
}

// Parse parses a source template. It returns an AST root, that can be compiled and evaluated.
func (c Config) Parse(source string) (ASTNode, error) {
	tokens := Scan(source, "")
	return c.parseChunks(tokens)
}

// Parse creates an AST from a sequence of Chunks.
func (c Config) parseChunks(chunks []Chunk) (ASTNode, error) { // nolint: gocyclo
	// a stack of control tag state, for matching nested {%if}{%endif%} etc.
	type frame struct {
		syntax BlockSyntax
		node   *ASTBlock
		ap     *[]ASTNode
	}
	var (
		g         = c.Grammar()
		root      = &ASTSeq{}      // root of AST; will be returned
		ap        = &root.Children // newly-constructed nodes are appended here
		sd        BlockSyntax      // current block syntax definition
		bn        *ASTBlock        // current block node
		stack     []frame          // stack of blocks
		rawTag    *ASTRaw          // current raw tag
		inComment = false
		inRaw     = false
	)
	for _, ch := range chunks {
		switch {
		// The parser needs to know about comment and raw, because tags inside
		// needn't match each other e.g. {%comment%}{%if%}{%endcomment%}
		// TODO is this true?
		case inComment:
			if ch.Type == TagChunkType && ch.Name == "endcomment" {
				inComment = false
			}
		case inRaw:
			if ch.Type == TagChunkType && ch.Name == "endraw" {
				inRaw = false
			} else {
				rawTag.slices = append(rawTag.slices, ch.Source)
			}
		case ch.Type == ObjChunkType:
			expr, err := expression.Parse(ch.Args)
			if err != nil {
				return nil, err
			}
			*ap = append(*ap, &ASTObject{ch, expr})
		case ch.Type == TextChunkType:
			*ap = append(*ap, &ASTText{Chunk: ch})
		case ch.Type == TagChunkType:
			if cs, ok := g.BlockSyntax(ch.Name); ok {
				switch {
				case ch.Name == "comment":
					inComment = true
				case ch.Name == "raw":
					inRaw = true
					rawTag = &ASTRaw{}
					*ap = append(*ap, rawTag)
				case cs.RequiresParent() && (sd == nil || !cs.CanHaveParent(sd)):
					suffix := ""
					if sd != nil {
						suffix = "; immediate parent is " + sd.TagName()
					}
					return nil, parseErrorf("%s not inside %s%s", ch.Name, strings.Join(cs.ParentTags(), " or "), suffix)
				case cs.IsBlockStart():
					push := func() {
						stack = append(stack, frame{syntax: sd, node: bn, ap: ap})
						sd, bn = cs, &ASTBlock{Chunk: ch, syntax: cs}
						*ap = append(*ap, bn)
					}
					push()
					ap = &bn.Body
				case cs.IsBranch():
					n := &ASTBlock{Chunk: ch, syntax: cs}
					bn.Branches = append(bn.Branches, n)
					ap = &n.Body
				case cs.IsBlockEnd():
					pop := func() {
						f := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						sd, bn, ap = f.syntax, f.node, f.ap
					}
					pop()
				default:
					panic("unexpected block type")
				}
			} else if td, ok := c.FindTagDefinition(ch.Name); ok {
				f, err := td(ch.Args)
				if err != nil {
					return nil, err
				}
				*ap = append(*ap, &ASTFunctional{ch, f})
			} else {
				return nil, parseErrorf("unknown tag: %s", ch.Name)
			}
		}
	}
	if bn != nil {
		return nil, parseErrorf("unterminated %s tag at %s", bn.Name, bn.SourceInfo)
	}
	return root, nil
}