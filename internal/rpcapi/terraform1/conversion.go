// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package terraform1

import (
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform/internal/lang/marks"
	"github.com/hashicorp/terraform/internal/plans"
	"github.com/hashicorp/terraform/internal/stacks/stackaddrs"
	"github.com/zclconf/go-cty/cty"
)

// This file contains some hand-written type conversion helpers to complement
// the generated stubs.

// ChangeTypesForPlanAction returns the [ChangeType] sequence that corresponds
// to the given plan action, or an error if there is no known equivalent.
func ChangeTypesForPlanAction(action plans.Action) ([]ChangeType, error) {
	switch action {
	case plans.NoOp:
		return nil, nil
	case plans.Create:
		return []ChangeType{ChangeType_CREATE}, nil
	case plans.Read:
		return []ChangeType{ChangeType_READ}, nil
	case plans.Update:
		return []ChangeType{ChangeType_UPDATE}, nil
	case plans.Delete:
		return []ChangeType{ChangeType_DELETE}, nil
	case plans.DeleteThenCreate:
		return []ChangeType{ChangeType_DELETE, ChangeType_CREATE}, nil
	case plans.CreateThenDelete:
		return []ChangeType{ChangeType_CREATE, ChangeType_DELETE}, nil
	default:
		return nil, fmt.Errorf("unsupported action %s", action)
	}
}

// NewDynamicValue constructs a [DynamicValue] message object from a
// [plans.DynamicValue], which is Terraform Core's typical in-memory
// representation of an already-serialized dynamic value.
//
// The plans package represents value marks (including "sensitive") as a
// separate field in [plans.ChangeSrc] rather than as part of the value
// itself, so callers must also provide that separate [cty.PathValueMarks]
// value if encoding a value that might have sensitive elements.
func NewDynamicValue(from plans.DynamicValue, markses []cty.PathValueMarks) *DynamicValue {
	// plans.DynamicValue is always MessagePack-serialized today, so we'll
	// just write its bytes into the field for msgpack serialization
	// unconditionally. If plans.DynamicValue grows to support different
	// serialization formats in future we will need some additional logic here.
	ret := &DynamicValue{
		Msgpack: []byte(from),
	}

	if len(markses) != 0 {
		ret.Sensitive = make([]*AttributePath, 0, len(markses))
		for _, pathMarks := range markses {
			if _, exists := pathMarks.Marks[marks.Sensitive]; exists {
				ret.Sensitive = append(ret.Sensitive, NewAttributePath(pathMarks.Path))
			}
		}
	}

	return ret
}

// NewAttributePath constructs an [AttributePath] message object from
// a [cty.Path] value.
func NewAttributePath(from cty.Path) *AttributePath {
	ret := &AttributePath{}
	if len(from) == 0 {
		return ret
	}
	ret.Steps = make([]*AttributePath_Step, len(from))
	for i, step := range from {
		switch step := step.(type) {
		case cty.GetAttrStep:
			ret.Steps[i] = &AttributePath_Step{
				Selector: &AttributePath_Step_AttributeName{
					AttributeName: step.Name,
				},
			}
		case cty.IndexStep:
			k := step.Key
			// Although the key is cty.Value, in practice it should typically
			// be constrained only to known and non-null strings and numbers.
			// If we encounter anything else then we'll just abort and return
			// a truncated path, since the only way other values should be
			// able to appear is if we're traversing through a set, and we
			// typically avoid doing that in callers by truncating the path
			// at the same point anyway. (Note that marked values -- one of
			// our main uses for AttributePath -- cannot exist inside
			// sets anyway, so that case can't arise there.)
			if k.IsNull() || !k.IsKnown() {
				k = cty.DynamicVal // to force falling into the default case for the switch below
			}

			switch k.Type() {
			case cty.String:
				ret.Steps[i] = &AttributePath_Step{
					Selector: &AttributePath_Step_ElementKeyString{
						ElementKeyString: k.AsString(),
					},
				}
			case cty.Number:
				// We require an integer in int64 range. We might not get that
				// in the unlikely event that this is a traversal through a
				// cty.Set(cty.Number), since any number would be valid in
				// principle for that case.
				bf := k.AsBigFloat()
				idx, acc := bf.Int64()
				if acc != big.Exact {
					ret.Steps = ret.Steps[:i]
					return ret
				}
				ret.Steps[i] = &AttributePath_Step{
					Selector: &AttributePath_Step_ElementKeyInt{
						ElementKeyInt: idx,
					},
				}
			default:
				ret.Steps = ret.Steps[:i]
				return ret
			}
		default:
			// Should not get here because the above should be exhaustive for
			// all cty.PathStep implementations.
			panic(fmt.Sprintf("path has unsupported step type %T", step))
		}
	}
	return ret
}

func NewResourceInstanceInStackAddr(addr stackaddrs.AbsResourceInstance) *ResourceInstanceInStackAddr {
	return &ResourceInstanceInStackAddr{
		ComponentInstanceAddr: addr.Component.String(),
		ResourceInstanceAddr:  addr.Item.String(),
	}
}

func NewResourceInstanceObjectInStackAddr(addr stackaddrs.AbsResourceInstanceObject) *ResourceInstanceObjectInStackAddr {
	return &ResourceInstanceObjectInStackAddr{
		ComponentInstanceAddr: addr.Component.String(),
		ResourceInstanceAddr:  addr.Item.ResourceInstance.String(),
		DeposedKey:            addr.Item.DeposedKey.String(),
	}
}
