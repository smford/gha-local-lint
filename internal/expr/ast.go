package expr

import (
	"fmt"
	"strings"
)

type Node interface {
	node()
	String() string
}

type LiteralNode struct {
	Value any // string, float64, bool, nil
}

func (n *LiteralNode) node() {}
func (n *LiteralNode) String() string {
	switch v := n.Value.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", v)
	}
}

type IdentNode struct {
	Name string
}

func (n *IdentNode) node() {}
func (n *IdentNode) String() string {
	return n.Name
}

type MemberNode struct {
	Object   Node
	Property string
}

func (n *MemberNode) node() {}
func (n *MemberNode) String() string {
	return fmt.Sprintf("%s.%s", n.Object.String(), n.Property)
}

type IndexNode struct {
	Object Node
	Index  Node
}

func (n *IndexNode) node() {}
func (n *IndexNode) String() string {
	return fmt.Sprintf("%s[%s]", n.Object.String(), n.Index.String())
}

type UnaryNode struct {
	Op   string
	Expr Node
}

func (n *UnaryNode) node() {}
func (n *UnaryNode) String() string {
	return fmt.Sprintf("%s%s", n.Op, n.Expr.String())
}

type BinaryNode struct {
	Op    string
	Left  Node
	Right Node
}

func (n *BinaryNode) node() {}
func (n *BinaryNode) String() string {
	return fmt.Sprintf("(%s %s %s)", n.Left.String(), n.Op, n.Right.String())
}

type CallNode struct {
	Name string
	Args []Node
}

func (n *CallNode) node() {}
func (n *CallNode) String() string {
	var argStrs []string
	for _, a := range n.Args {
		argStrs = append(argStrs, a.String())
	}
	return fmt.Sprintf("%s(%s)", n.Name, strings.Join(argStrs, ", "))
}

type StarNode struct{}

func (n *StarNode) node() {}
func (n *StarNode) String() string {
	return "*"
}
