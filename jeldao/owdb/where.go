package owdb

// TODO: names of types in this package have changed numerous times and the docs
// need to be properly updated to reflect them.
// TODO: terminology change. don't call it "group" mode and condition mode, call
// it "operation" mode and "condition" mode. Actually, please please please!
// come up with a betta name than "condition" mode. "single"? "leaf" vs "tree"?
// "direct"? idk.

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

var (
	// rng is used for creating random names of Criterion structs created with
	// Meets.
	rng *rand.Rand = rand.New(rand.NewSource(time.Now().Unix()))
)

// Limits gives the max and minimum for a value. It is used for query planning
// based on a filter provided. It can be applied even to normally non-comparable
// types, as long as Min <= Max according to some known ordering.
//
// If Min or Max are set to nil, they should be considered as "no limit" on that
// side of things.
type Limits[E any] struct {
	Min *E
	Max *E
}

// IsImpossible returns whether the limits are impossible for any value to fall
// between, due to both being set and Min being greater than Max. The function
// gt tells whether e1 is greater than e2.
func (lim Limits[E]) IsImpossible(gt func(e1, e2 E) bool) bool {
	if lim.Min == nil || lim.Max == nil {
		return false
	}

	return gt(*lim.Min, *lim.Max)
}

// Contains returns whether the limits include the given item pt. This will be
// checked both by comparison and greater-than checks using the provided eq and
// gt functions respectively.
//
// Limits is said to contain a point when it falls between the two bounds.
// Undefined bounds are treated as infinity in that direction; a nil Min is
// negative infinity, and a nil Max is positive infinity.
func (lim Limits[E]) Contains(pt E, eq func(e1, e2 E) bool, gt func(e1, e2 E) bool) bool {
	return (lim.Min == nil || (eq(pt, *lim.Min) || gt(pt, *lim.Min))) && (lim.Max == nil || (eq(pt, *lim.Max) || gt(*lim.Max, pt)))
}

// Narrow returns a new Limits whose Max is the lesser of lim and other, and
// whose Min is the greater of lim and other. A nil Min or Max is always
// overcome by a non-nil Min or Max. The function gt must be provided to give
// how to tell that the first operand is greater than the second.
func (lim Limits[E]) Narrow(other Limits[E], gt func(e1, e2 E) bool) Limits[E] {
	// if either is impossible, so is the narrowed range.
	if lim.IsImpossible(gt) || other.IsImpossible(gt) {
		return lim
	}

	newLim := Limits[E]{}

	// both are possible. take the narrowed range.
	newLim.Max = lim.Max
	if newLim.Max == nil || (other.Max != nil && gt(*newLim.Max, *other.Max)) {
		newLim.Max = other.Max
	}
	newLim.Min = lim.Min
	if newLim.Min == nil || (other.Min != nil && gt(*other.Min, *newLim.Min)) {
		newLim.Min = other.Min
	}

	return newLim
}

// Widen returns a new Limits whose Max is the greater of lim and other, and
// whose Min is the lesser of lim and other. A nil Min or Max will always
// overcome a non-nil one, as they represent infinite coverage on that end. The
// function gt must be provided to give how to tell that the first operand is
// greater than the second.
func (lim Limits[E]) Widen(other Limits[E], gt func(e1, e2 E) bool) Limits[E] {
	// if either is impossible, discard it and take the other.
	if lim.IsImpossible(gt) {
		return other
	} else if other.IsImpossible(gt) {
		return lim
	}

	newLim := Limits[E]{}

	// both are possible. take the widened range.
	newLim.Max = lim.Max
	if newLim.Max != nil && (other.Max == nil || gt(*other.Max, *newLim.Max)) {
		newLim.Max = other.Max
	}
	newLim.Min = lim.Min
	if newLim.Min != nil && (other.Min == nil || gt(*newLim.Min, *other.Min)) {
		newLim.Min = other.Min
	}

	return newLim
}

// Operator is an operation that is applied to all operands of a Where in group
// mode. NOT is a unary operator and can only apply to a single operand;
// attempting to evaluate it with more than one operand will result in a panic.
type Operator int

const (
	AND Operator = iota
	OR
	NOT
)

func (op Operator) String() string {
	switch op {
	case AND:
		return "AND"
	case OR:
		return "OR"
	case NOT:
		return "NOT"
	default:
		return fmt.Sprintf("Operator(%d)", op)
	}
}

// Filter is an interface that is implemented by all types that can be
// combined into a Where. Because a Where requires that anything combined with
// it itself be a Where, this interface indicates that the type can be converted
// to one and then combined with it. Additionally, Filter supports the
// creation of a Where via the addition of an operator and any applicable
// operands, selected via And, Or, or Negate.
type Filter interface {
	Node() FilterNode
	And(clause Filter, clauses ...Filter) FilterNode
	Or(clause Filter, clauses ...Filter) FilterNode
	Negate() FilterNode

	TimeIndexLimits() Limits[time.Time]
	Matches(h Hit) bool
}

// FilterNode is a set of conditions to match against all Hits that an operation
// is to apply to. It is either a "condition"-mode FilterNode, which includes
// specific criteria, or a "group"-mode FilterNode, which combines criterion-mode
// WhereNodes with binary operators. A FilterNode cannot be both. If And and Or
// are used to combine WhereNodes and Conditions, this will be handled
// automatically.
//
// Cond determines whether the FilterNode is condition mode or group mode. If it
// is set to a non-nil condition, the FilterNode is in condition mode, Group and
// Op are ignored. If Cond is set to nil, the FilterNode is in group mode and
// will test a hit against all Wheres in Group, combined with Op.
//
// An Op of NOT in group mode will use only a single operand from Group. If a
// FilterNode is evaluated with Op of NOT and multiple operands in Group, all others
// are ignored. A FilterNode in group mode with an operand of NOT will return false
// for all Hits passed to Matches.
//
// The zero-value is a ready to use FilterNode in group mode that will match all
// Hits.
type FilterNode struct {
	Cond  *Where
	Op    Operator
	Group []FilterNode
}

// String prints out the string representation of the FilterNode. Two
// FilterNodes should be considered exactly equivalent if they produce the same
// string, as they will match and fail to match on the same inputs.
func (n FilterNode) String() string {
	if !n.IsOperation() {
		return n.Cond.String()
	}

	var delim string

	var sb strings.Builder
	if n.Op != NOT {
		delim = " " + n.Op.String() + " "
		for _, child := range n.Group {
			if sb.Len() > 0 {
				sb.WriteString(delim)
			}
			sb.WriteRune('(')
			sb.WriteString(child.String())
			sb.WriteRune(')')
		}
	} else {
		delim = n.Op.String() + " "
		sb.WriteString(delim)
		sb.WriteRune('(')
		sb.WriteString(n.Group[0].String())
		sb.WriteRune(')')
	}

	return sb.String()
}

// Simplify returns a FilterNode that represents the same logic as this one but
// with any redundant operations removed (such as a double NOT). If n is already
// simplest terms, it will return itself.
func (n FilterNode) Simplify() FilterNode {
	if !n.IsOperation() {
		// no more simplification to do, just return self.
		return n
	}

	var simplified FilterNode

	// in future, probs could try to combine equiv AND and OR terms, but that's
	// a big topic so not getting into it rn.

	if n.Op == NOT {
		// redundant NOT check
		if n.Group[0].IsOperation() && n.Group[0].Op == NOT {
			// double-NOT detected. Reach down and get the target of the 2nd NOT and
			// return that instead of self.

			target := n.Group[0].Group[0]
			return target.Simplify()
		}

		simplified = FilterNode{
			Op:    NOT,
			Group: []FilterNode{n.Group[0].Simplify()},
		}

		return simplified
	}

	// binary, non-NOT operators
	// need to simplify each AND.
	simplified = FilterNode{
		Op:    n.Op,
		Group: make([]FilterNode, len(n.Group)),
	}
	for i := range n.Group {
		simplified.Group[i] = n.Group[i].Simplify()
	}

	return simplified
}

// TimeIndexLimits returns the limits on the indexed Time field of Hits that
// this FilterNode would impose. If either end is "open", it will be nil. Both
// ends being open means this doesn't have any limits. Returned Limits should be
// checked with IsImpossible() to verify that they are possible before using.
func (n FilterNode) TimeIndexLimits() Limits[time.Time] {
	if !n.IsOperation() {
		// easy-peasy, delegate to the held Where
		lims := n.Cond.TimeIndexLimits()
		return lims
	}

	// first, simplify the FilterNode to eliminate reduntant NOTs
	n = n.Simplify()

	switch n.Op {
	case NOT:
		// we can't really get much more info out of a NOT; it should always
		// return INF, INF until we get a more sophisticated check.
		return Limits[time.Time]{}
	case AND:
		lims := n.Group[0].TimeIndexLimits()
		for _, child := range n.Group[1:] {
			// if any of these results in an impossible range, no need to do
			// the rest, they are impossible.
			if lims.IsImpossible(time.Time.After) {
				break
			}
			lims = lims.Narrow(child.TimeIndexLimits(), time.Time.After)
		}
	case OR:
		var lims Limits[time.Time]
		validIdx := -1

		// skip over any impossibles to get the first valid one
		for i := range n.Group {
			childLim := n.Group[i].TimeIndexLimits()
			if !childLim.IsImpossible(time.Time.After) {
				lims = childLim
				validIdx = i
				break
			}
		}

		if validIdx == -1 {
			// all operands are equally impossible so return the first
			return n.Group[0].TimeIndexLimits()
		}

		// otherwise, iterate over the rest and OR lims with each non-impossible
		// one
		for _, child := range n.Group[validIdx+1:] {
			childLim := child.TimeIndexLimits()
			if childLim.IsImpossible(time.Time.After) {
				continue
			}

			lims = lims.Widen(childLim, time.Time.After)
		}

		return lims
	default:
		panic(fmt.Sprintf("unknown operation: %s", n.Op))
	}

	return Limits[time.Time]{}
}

// Node returns the FilterNode itself. It is included for implementation of
// Filter.
func (n FilterNode) Node() FilterNode {
	return n
}

// IsOperation returns whether the FilterNode represents a grouped operation,
// that is, one or more operands that an operator is applied to. A FilterNode
// that represents this is said to be in "group" mode as opposed to "condition"
// mode, because its Group (and Op) members are used to check whether it matches
// some input as opposed to the Where in the FilterNode's Cond member.
//
// A FilterNode with Cond set to nil is considered an operation, regardless of
// the values of Group and Op. Likewise, a FilterNode with Cond set to true is
// conidered not an operation (though the conceptual line gets a bit blurry when
// Cond contains multiple criteria, which strictly speaking are treated as
// though they are AND'd together).
//
// TODO: above parenthetical not needed, move that to pkg docs
//
// If IsOperation returns false, then the FilterNode is in condition mode.
func (n FilterNode) IsOperation() bool {
	return n.Cond == nil
}

// And returns a new Where that that matches only those Hits that match all of
// the given combined conditions. Multiple Combiners can be given to have them
// all be a part of the same sequence of Ands, and will be evaluated in order.
//
// Calling this returns a Where that represents the composite condition given
// by (w && co1 ... && coN).
func (n FilterNode) And(com Filter, coms ...Filter) FilterNode {
	// if we are already an AND, we could combine them, but this breaks the
	// contract of building a new Where, so we do not.

	newN := FilterNode{
		Op:    AND,
		Group: make([]FilterNode, len(coms)+2),
	}

	// add self and first operand
	newN.Group[0] = n
	newN.Group[1] = com.Node()

	// and any others given
	for i := 0; i < len(coms); i++ {
		var extraCond = coms[i]
		newN.Group[2+i] = extraCond.Node()
	}

	return newN
}

// Or returns a new Where that that matches all Hits that match at least one of
// the given combined conditions. Multiple Combiners can be given to have them
// all be a part of the same sequence of Ors, and will be evaluated in order.
//
// Calling this returns a Where that represents the composite condition given
// by (w || co1 ... || coN).
func (n FilterNode) Or(com Filter, coms ...Filter) FilterNode {
	// if we are already an OR, we could combine them, but this breaks the
	// contract of building a new Where, so we do not.

	newN := FilterNode{
		Op:    OR,
		Group: make([]FilterNode, len(coms)+2),
	}

	// add self and first operand
	newN.Group[0] = n
	newN.Group[1] = com.Node()

	// and any others given
	for i := 0; i < len(coms); i++ {
		var extraCond = coms[i]
		newN.Group[2+i] = extraCond.Node()
	}

	return newN
}

// Negate returns a Where that matches only those Hits that do *not* match w.
//
// Calling this returns a Where that represents the composite condition given
// by !w.
func (n FilterNode) Negate() FilterNode {
	return FilterNode{Group: []FilterNode{n}, Op: NOT}
}

// Matches returns whether the given Hit matches this Where clause.
func (n FilterNode) Matches(h Hit) bool {
	var match bool

	if n.Cond != nil {
		// condition mode
		match = n.Cond.Matches(h)
	} else {

		// group mode

		// which binary operation are we in?
		switch n.Op {
		case AND:
			match = true
			for _, childCond := range n.Group {
				if !childCond.Matches(h) {
					// short-circuit
					match = false
					break
				}
			}
		case OR:
			match = false
			for _, childCond := range n.Group {
				if childCond.Matches(h) {
					// short-circuit
					match = true
					break
				}
			}
		case NOT:
			match = false
			if len(n.Group) > 0 {
				match = !n.Group[0].Matches(h)
			}
		default:
			panic(fmt.Sprintf("undefined operator in Where clause: %v", n.Op))
		}
	}

	return match
}

// Where is a set of criteria that a Hit can be matched against. It may have
// up to one check per property of a Hit.
type Where struct {
	Time          Criterion[time.Time]
	Host          Criterion[string]
	Resource      Criterion[string]
	ClientAddress Criterion[net.IP]
	ClientCountry Criterion[string]
	ClientCity    Criterion[string]
}

// Matches returns whether the criteria defined by this Where match the
// given Hit.
func (w Where) Matches(h Hit) bool {
	if w.Time.Meets != nil {
		if !w.Time.Meets(h.Time) {
			return false
		}
	}

	if w.Host.Meets != nil {
		if !w.Host.Meets(h.Host) {
			return false
		}
	}

	if w.Resource.Meets != nil {
		if !w.Resource.Meets(h.Resource) {
			return false
		}
	}

	if w.ClientAddress.Meets != nil {
		if !w.ClientAddress.Meets(h.Client.Address) {
			return false
		}
	}

	if w.ClientCity.Meets != nil {
		if !w.ClientCity.Meets(h.Client.City) {
			return false
		}
	}

	if w.ClientCountry.Meets != nil {
		if !w.ClientCountry.Meets(h.Client.Country) {
			return false
		}
	}

	return true
}

// TimeIndexLimits returns the limits on the indexed Time field of Hits that
// this where would impose. If either end is "open", it will be nil. Both ends
// being open means this doesn't have a lowest one.
func (w Where) TimeIndexLimits() Limits[time.Time] {
	var allLimits Limits[time.Time]
	var set bool

	if w.Time.Meets != nil {
		if !set {
			allLimits = w.Time.EstLimits
		} else {
			allLimits = allLimits.Widen(w.Time.EstLimits, time.Time.After)
		}
		set = true
	}

	return allLimits
}

// String prints out the string representation of this Where. Two Where structs
// that return the same values from String() should be considered exactly
// equivalent, as they will produce identical output from their respective
// Matches.
func (w Where) String() string {
	var sb strings.Builder

	if w.Time.Meets != nil {
		if sb.Len() > 0 {
			sb.WriteRune(' ')
			sb.WriteString(AND.String())
			sb.WriteRune(' ')
		}
		sb.WriteString(w.Time.FilledString("time"))
	}

	if sb.Len() < 1 {
		// if this Where is empty of all criteria, this where is effectively
		// just 'true'
		return "TRUE"
	}
	return sb.String()
}

// And combines both this and any other Where into a single WhereNode clause
// that matches only those Hits that match all of the Wheres. Multiple
// Wheres can be given to have them all be a part of the same sequence of
// Ands, and will be evaluated in order.
//
// Calling this returns a Where that represents the composite condition given
// by (cond && com ... && comN).
func (w Where) And(com Filter, coms ...Filter) FilterNode {
	n := FilterNode{
		Op:    AND,
		Group: make([]FilterNode, len(coms)+2),
	}

	// add self and first operand
	n.Group[0] = w.Node()
	n.Group[1] = com.Node()

	// and any others given
	for i := 0; i < len(coms); i++ {
		var extraCom = coms[i]
		n.Group[2+i] = extraCom.Node()
	}

	return n
}

// And combines both this and any other Wheres given into a single WhereNode
// clause that matches only those Hits that match at least one of the
// Wheres. Multiple Conditions can be given to have them all be a part of the
// same sequence of Ors, and will be evaluated in order.
//
// Calling this returns a WhereNode that represents the composite condition given
// by (cond || com ... || comN).
func (w Where) Or(com Filter, coms ...Filter) FilterNode {
	n := FilterNode{
		Op:    OR,
		Group: make([]FilterNode, len(coms)+2),
	}

	// add self and first operand
	n.Group[0] = w.Node()
	n.Group[1] = com.Node()

	// and any others given
	for i := 0; i < len(coms); i++ {
		var extraCom = coms[i]
		n.Group[2+i] = extraCom.Node()
	}

	return n
}

// Negate returns a WhereNode that matches only those Hits that do *not* match
// the Where.
//
// Calling this returns a WhereNode that represents the composite condition given
// by !cond.
func (w Where) Negate() FilterNode {
	return FilterNode{Group: []FilterNode{w.Node()}, Op: NOT}
}

// AsWhere returns a new Condition-mode Where that matches Hits against this
// condition. It is included to implement WhereCombiner.
func (cond Where) Node() FilterNode {
	return FilterNode{Cond: &cond}
}

func Not(f Filter) FilterNode {
	return f.Negate()
}
