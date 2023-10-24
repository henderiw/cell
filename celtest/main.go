package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/golang/glog"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"sigs.k8s.io/yaml"
)

var dnn1 = `
apiVersion: req.nephio.org/v1alpha1
kind: DataNetwork
metadata:
  name: dnn1
spec:
  networkInstance:
    name: vpc-internet
  pools:
  - name: pool11
    prefixLength: 8
  - name: pool12
    prefixLength: 8
`

var dnn2 = `
apiVersion: req.nephio.org/v1alpha1
kind: DataNetwork
metadata:
  name: dnn2
spec:
  networkInstance:
    name: vpc-internet
  pools:
  - name: pool21
    prefixLength: 8
  - name: pool22
    prefixLength: 8
`

var ifce = `
apiVersion: req.nephio.org/v1alpha1
kind: Interface
metadata:
  name: n3
spec:
  networkInstance:
    name: vpc-ran
  cniType: sriov
  attachmentType: vlan
  ipFamilyPolicy: dualstack
`

func main() {

	var dnn1Yaml any
	if err := yaml.Unmarshal([]byte(dnn1), &dnn1Yaml); err != nil {
		panic(err)
	}
	var dnn2Yaml any
	if err := yaml.Unmarshal([]byte(dnn2), &dnn2Yaml); err != nil {
		panic(err)
	}

	dnns := []any{
		dnn1Yaml,
		dnn2Yaml,
	}

	var itfce any
	if err := yaml.Unmarshal([]byte(ifce), &itfce); err != nil {
		panic(err)
	}

	env, err := cel.NewEnv(
		cel.Variable("var_dnns", cel.ListType(cel.DynType)),
		cel.Variable("a_string", cel.StringType),
		cel.Variable("b_string", cel.StringType),
		cel.Variable("l1", cel.ListType(cel.DynType)),
		cel.Variable("l2", cel.ListType(cel.DynType)),
		cel.Variable("var_itfce", cel.ListType(cel.DynType)),
		cel.Function("concat",
			cel.MemberOverload("string_concat",
				[]*cel.Type{cel.ListType(cel.StringType), cel.StringType},
				cel.StringType,
				cel.BinaryBinding(func(elems ref.Val, sep ref.Val) ref.Val {
					return stringOrError(concat(elems.(traits.Lister), string(sep.(types.String))))
				}),
			),
		),
		Lists(),
	)
	if err != nil {
		panic(err)
	}
	/*
		ast, issues := env.Compile(`var_dnns.all(x, size(x.spec.pools) > 3)`)
		if issues != nil && issues.Err() != nil {
			panic(err)
		}
	*/

	exprs := []string{
		`[a_string, b_string].concat('-')`,
		`l1.listsconcat(l2)`,
		`var_dnns[0]`,
		`var_itfce[0].spec.networkInstance.name != 'default' && var_itfce[0].spec.attachmentType == 'vlan' ? 1 : 0`,
		`var_itfce[0].spec.networkInstance.name != 'default' && (var_itfce[0].spec.ipFamilyPolicy == 'dualstack' || var_itfce[0].spec.ipFamilyPolicy == 'ipv4Only') ? 1 : 0`,
	}

	for _, expr := range exprs {
		ast, issues := env.Compile(expr)
		if issues != nil && issues.Err() != nil {
			panic(issues.Err())
		}
		prg, err := env.Program(ast)
		if err != nil {
			panic(err)
		}
		out, _, err := prg.Eval(map[string]interface{}{
			"var_dnns": dnns,
			"var_itfce": []any{itfce},
			"a_string": "alpha",
			"b_string": "beta",
			"l1":       []string{"a"},
			"l2":       []string{"b", "c", "d"},
		})
		if err != nil {
			panic(err)
		}
		fmt.Println(valueToJSON(out)) // 'true'
		//fmt.Println(details) // 'true'
	}

}

func concat(strs traits.Lister, separator string) (string, error) {
	sz := strs.Size().(types.Int)
	var sb strings.Builder
	for i := types.Int(0); i < sz; i++ {
		if i != 0 {
			sb.WriteString(separator)
		}
		elem := strs.Get(i)
		str, ok := elem.(types.String)
		if !ok {
			return "", fmt.Errorf("join: invalid input: %v", elem)
		}
		sb.WriteString(string(str))
	}
	return sb.String(), nil
}

func stringOrError(str string, err error) ref.Val {
	if err != nil {
		return types.NewErr(err.Error())
	}
	return types.String(str)
}

func Lists() cel.EnvOption {
	return cel.Lib(listslib{})
}

type listslib struct{}

// LibraryName implements the SingletonLibrary interface method.
func (listslib) LibraryName() string {
	return "cel.lib.ext.lists"
}

// ProgramOptions implements the Library interface method.
func (listslib) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

// CompileOptions implements the Library interface method.
func (listslib) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("listsconcat",
			cel.MemberOverload("lists_concat",
				[]*cel.Type{cel.AnyType, cel.AnyType},
				cel.ListType(cel.AnyType),
				cel.BinaryBinding(func(x1 ref.Val, x2 ref.Val) ref.Val {
					l1 := x1.(traits.Lister)
					l2 := x2.(traits.Lister)
					x2 = l1.Add(l2)
					return x2
				}),
			),
		),
	}
}

// valueToJSON converts the CEL type to a protobuf JSON representation and
// marshals the result to a string.
func valueToJSON(val ref.Val) string {
	v, err := val.ConvertToNative(reflect.TypeOf(&structpb.Value{}))
	if err != nil {
		glog.Exit(err)
	}
	marshaller := protojson.MarshalOptions{Indent: "    "}
	bytes, err := marshaller.Marshal(v.(proto.Message))
	if err != nil {
		glog.Exit(err)
	}
	return string(bytes)
}
