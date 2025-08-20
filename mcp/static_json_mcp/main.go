package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.SetFlags(0)
	filePath := flag.String("file", "", "Path to JSON file (must contain array)")
	toolName := flag.String("tool", "get_data", "MCP tool name to expose")
	serverName := flag.String("name", "MockDataTool", "Name of the MCP server")
	flag.Parse()

	if *filePath == "" {
		log.Fatal("--file is required")
	}

	input, err := os.ReadFile(*filePath)
	if err != nil {
		log.Fatalf("failed to read file: %v", err)
	}

	var jsonArray []any
	if err := json.Unmarshal(input, &jsonArray); err != nil {
		log.Fatalf("JSON must be an array of objects: %v", err)
	}

	outputStruct := buildStructFromJSONSample(jsonArray)

	srv := server.NewMCPServer(*serverName, "v0.0.1")
	srv.AddTool(
		mcp.NewTool(*toolName,
			mcp.WithDescription("Returns paged JSON data with inferred raw schema"),
			WithOutputSchema(outputStruct),
			mcp.WithNumber("page", mcp.Description("The page to read. Defaults to 0")),
			mcp.WithNumber("page_size", mcp.Description("The page size to read. Defaults to 10")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			page := 0
			pageSize := 10

			if val := req.GetInt("page", -1); val != -1 {
				page = val
			}
			if val := req.GetInt("page_size", -1); val != -1 {
				pageSize = val
			}

			paged := paginate(jsonArray, page, pageSize)
			out, err := json.Marshal(paged)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal paged data: %w", err)
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: string(out),
					},
				},
			}, nil
		},
	)

	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func paginate(array []any, page, pageSize int) []any {
	start := page * pageSize
	if start >= len(array) {
		return nil
	}

	end := start + pageSize
	if end > len(array) {
		end = len(array)
	}

	return array[start:end]
}

func buildStructFromJSONSample(sample []any) any {
	m := map[string]any{}
	for _, entry := range sample {
		for k, v := range entry.(map[string]any) {
			m[k] = v
		}
	}

	var fields []reflect.StructField

	for key, val := range m {
		fields = append(fields, reflect.StructField{
			Name: exportableFieldName(key),
			Type: inferReflectType(val),
			Tag:  reflect.StructTag(fmt.Sprintf(`json:"%s"`, key)),
		})
	}

	typ := reflect.StructOf(fields)
	return reflect.New(typ).Interface()
}

func exportableFieldName(key string) string {
	parts := strings.FieldsFunc(key, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for i := range parts {
		parts[i] = strings.Title(parts[i])
	}
	return strings.Join(parts, "")
}

func inferReflectType(val any) reflect.Type {
	switch val := val.(type) {
	case string:
		return reflect.TypeOf("")
	case bool:
		return reflect.TypeOf(true)
	case float64:
		if float64(int64(val)) == val {
			return reflect.TypeOf(int64(0))
		}
		return reflect.TypeOf(float64(0))
	case []any:
		return reflect.TypeOf([]any{})
	case map[string]any:
		return reflect.TypeOf(map[string]any{})
	default:
		return reflect.TypeOf(nil)
	}
}

// This is a copy of mcp.WithOutputSchema that is not generic.
func WithOutputSchema(zero any) mcp.ToolOption {
	return func(t *mcp.Tool) {
		// Generate schema using invopop/jsonschema library
		// Configure reflector to generate clean, MCP-compatible schemas
		reflector := jsonschema.Reflector{
			DoNotReference:            true, // Removes $defs map, outputs entire structure inline
			Anonymous:                 true, // Hides auto-generated Schema IDs
			AllowAdditionalProperties: true, // Removes additionalProperties: false
		}
		schema := reflector.Reflect(zero)

		// Clean up schema for MCP compliance
		schema.Version = "" // Remove $schema field

		// Convert to raw JSON for MCP
		mcpSchema, err := json.Marshal(schema)
		if err != nil {
			// Skip and maintain backward compatibility
			return
		}

		t.RawOutputSchema = json.RawMessage(mcpSchema)
	}
}
