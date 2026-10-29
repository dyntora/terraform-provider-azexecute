package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Reference pages must cover the registered schema, including nested attributes.
// Behavioral correctness still requires review and lifecycle tests.
func TestDocumentationCoversRegisteredSchemas(t *testing.T) {
	ctx := context.Background()
	p := New("test")()
	index, err := os.ReadFile(filepath.Join("..", "..", "docs", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, path string, schemaType tftypes.Type) {
		t.Helper()
		content, err := os.ReadFile(filepath.Join("..", "..", "docs", path))
		if err != nil {
			t.Fatal(err)
		}
		if !utf8.Valid(content) || strings.ContainsRune(string(content), '\uFFFD') {
			t.Errorf("%s contains invalid or damaged UTF-8", path)
		}
		if path != "index.md" && !strings.Contains(string(index), "("+filepath.ToSlash(path)+")") {
			t.Errorf("index.md does not link to %s", path)
		}
		var visit func(tftypes.Type, string)
		visit = func(typ tftypes.Type, prefix string) {
			switch typ := typ.(type) {
			case tftypes.Object:
				for name, child := range typ.AttributeTypes {
					if !strings.Contains(string(content), "`"+name+"`") {
						t.Errorf("%s does not document `%s` (schema path %s%s)", path, name, prefix, name)
					}
					visit(child, prefix+name+".")
				}
			case tftypes.List:
				visit(typ.ElementType, prefix)
			case tftypes.Set:
				visit(typ.ElementType, prefix)
			case tftypes.Map:
				visit(typ.ElementType, prefix)
			}
		}
		visit(schemaType, "")
	}
	var providerSchema frameworkprovider.SchemaResponse
	p.Schema(ctx, frameworkprovider.SchemaRequest{}, &providerSchema)
	check(t, "index.md", providerSchema.Schema.Type().TerraformType(ctx))
	for _, factory := range p.Resources(ctx) {
		r := factory()
		var metadata resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "azexecute"}, &metadata)
		t.Run(metadata.TypeName+"_resource", func(t *testing.T) {
			var response resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &response)
			check(t, filepath.Join("resources", strings.TrimPrefix(metadata.TypeName, "azexecute_")+".md"), response.Schema.Type().TerraformType(ctx))
		})
	}
	for _, factory := range p.DataSources(ctx) {
		d := factory()
		var metadata datasource.MetadataResponse
		d.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "azexecute"}, &metadata)
		t.Run(metadata.TypeName+"_data_source", func(t *testing.T) {
			var response datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &response)
			check(t, filepath.Join("data-sources", strings.TrimPrefix(metadata.TypeName, "azexecute_")+".md"), response.Schema.Type().TerraformType(ctx))
		})
	}
}
