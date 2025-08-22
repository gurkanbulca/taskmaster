//go:generate go run generate.go

// ent/generate.go

package main

import (
	"log"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	err := entc.Generate("./schema", &gen.Config{
		Target:  "./generated", // Output to generated directory
		Package: "github.com/gurkanbulca/taskmaster/ent/generated",
		Features: []gen.Feature{
			gen.FeatureEntQL,
		},
	})
	if err != nil {
		log.Fatal("running ent codegen:", err)
	}
}
