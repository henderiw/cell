package main

import (
	"fmt"
	"log"

	"github.com/google/cel-go/cel"
)

func main() {
	env, err := cel.NewEnv(
		cel.Variable("name", cel.StringType),
		cel.Variable("group", cel.StringType),
	)
	if err != nil {
		log.Fatalf("program construction error: %s", err)
	}

	ast, issues := env.Compile(`name.startsWith("/groups/" + group)`)
	if issues != nil && issues.Err() != nil {
		log.Fatalf("type-check error: %s", issues.Err())
	}
	fmt.Println("ast", ast)
	prg, err := env.Program(ast)
	if err != nil {
		log.Fatalf("program construction error: %s", err)
	}
	// The `out` var contains the output of a successful evaluation.
	// The `details' var would contain intermediate evaluation state if enabled as
	// a cel.ProgramOption. This can be useful for visualizing how the `out` value
	// was arrive at.
	out, details, err := prg.Eval(map[string]interface{}{
		"name":  "/groups/acme.co/documents/secret-stuff",
		"group": "acme.co"})
	if err != nil {
		log.Fatalf("program construction error: %s", err)
	}
	fmt.Println(out)     // 'true'
	fmt.Println(details) // 'true'
}
