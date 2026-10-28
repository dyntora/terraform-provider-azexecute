package provider

import (
	"context"
	"encoding/json"
	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func groupTestModel() azgroupModel {
	return azgroupModel{ID: types.StringUnknown(), GroupID: types.Int64Unknown(), Name: types.StringValue("Team"), Description: types.StringNull(), Source: types.StringValue("auto"), ActualSource: types.StringUnknown(), DisplayName: types.StringUnknown(), EntraObjectID: types.StringUnknown(), Status: types.StringUnknown(), GroupType: types.StringNull(), Active: types.BoolValue(true), Color: types.StringNull(), Icon: types.StringNull(), DeleteEntraOnDestroy: types.BoolValue(false)}
}

func TestAZGroupDataSourcesReadWithoutAdopting(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" {
			t.Errorf("read data source issued %s", req.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/api/terraform/v1/groups/capabilities":
			_ = json.NewEncoder(w).Encode(azclient.GroupCapabilities{Enabled: true, CanCreateGroups: true, CreateGroupsInEntra: true})
		case "/api/terraform/v1/groups/lookup/17":
			_ = json.NewEncoder(w).Encode(azclient.Group{ResourceID: "00000000-0000-0000-0000-000000000000", GroupID: 17, DisplayName: "Local team", ActualSource: "Local", Status: "Ready", Configuration: azclient.GroupConfiguration{Name: "Local team", Source: "Local", IsActive: true}})
		default:
			t.Errorf("unexpected path %s", req.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "ignored", "token", nil, time.Second)
	d := &azgroupDataSource{client: api}
	sr := datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, &sr)
	if diag := sr.Schema.ValidateImplementation(ctx); diag.HasError() {
		t.Fatal(diag)
	}
	config := tfsdk.Config{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
	state := tfsdk.State{Schema: sr.Schema, Raw: config.Raw}
	m := groupTestModel()
	m.GroupID = types.Int64Value(17)
	if diag := state.Set(ctx, m); diag.HasError() {
		t.Fatal(diag)
	}
	config.Raw = state.Raw
	read := datasource.ReadResponse{State: state}
	d.Read(ctx, datasource.ReadRequest{Config: config}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	if diag := read.State.Get(ctx, &m); diag.HasError() {
		t.Fatal(diag)
	}
	if m.Name.ValueString() != "Local team" || m.ID.ValueString() != "00000000-0000-0000-0000-000000000000" {
		t.Fatal(m)
	}
	c := &groupCapabilitiesDataSource{client: api}
	cs := datasource.SchemaResponse{}
	c.Schema(ctx, datasource.SchemaRequest{}, &cs)
	if diag := cs.Schema.ValidateImplementation(ctx); diag.HasError() {
		t.Fatal(diag)
	}
	cr := datasource.ReadResponse{State: tfsdk.State{Schema: cs.Schema, Raw: tftypes.NewValue(cs.Schema.Type().TerraformType(ctx), nil)}}
	c.Read(ctx, datasource.ReadRequest{}, &cr)
	if cr.Diagnostics.HasError() {
		t.Fatal(cr.Diagnostics)
	}
	var policy groupCapabilitiesModel
	if diag := cr.State.Get(ctx, &policy); diag.HasError() {
		t.Fatal(diag)
	}
	if !policy.Enabled.ValueBool() || !policy.CanCreate.ValueBool() || policy.CanCreateLocal.ValueBool() || !policy.EntraDefault.ValueBool() {
		t.Fatal(policy)
	}
}

func TestAZGroupLifecycleImportAndAccessLoss(t *testing.T) {
	ctx := context.Background()
	var saved azclient.Group
	status := 200
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Header.Get("Authorization") != "Bearer token" {
			t.Error("missing bearer token")
		}
		if status != 200 {
			w.WriteHeader(status)
			return
		}
		switch req.Method {
		case "POST", "PUT":
			if !strings.Contains(req.URL.Path, "/import/") {
				if err := json.NewDecoder(req.Body).Decode(&saved.Configuration); err != nil {
					t.Error(err)
				}
				saved.ResourceID = req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
				saved.GroupID = 17
				saved.DisplayName = "AZX-" + saved.Configuration.Name
				saved.ActualSource = "Entra"
				saved.Status = "Ready"
			}
		case "DELETE":
			deletes++
			w.WriteHeader(204)
			return
		}
		_ = json.NewEncoder(w).Encode(saved)
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "ignored", "token", nil, time.Second)
	r := &azgroupResource{client: api}
	sr := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	if d := sr.Schema.ValidateImplementation(ctx); d.HasError() {
		t.Fatal(d)
	}
	empty := func() tfsdk.State {
		return tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
	}
	plan := func(m azgroupModel) tfsdk.Plan {
		p := tfsdk.Plan{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
		if d := p.Set(ctx, m); d.HasError() {
			t.Fatal(d)
		}
		return p
	}
	created := resource.CreateResponse{State: empty()}
	r.Create(ctx, resource.CreateRequest{Plan: plan(groupTestModel())}, &created)
	if created.Diagnostics.HasError() {
		t.Fatal(created.Diagnostics)
	}
	var m azgroupModel
	if d := created.State.Get(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	if m.GroupID.ValueInt64() != 17 || m.DisplayName.ValueString() != "AZX-Team" || m.Name.ValueString() != "Team" {
		t.Fatal(m)
	}
	m.Name = types.StringValue("Changed")
	m.DeleteEntraOnDestroy = types.BoolValue(true)
	updated := resource.UpdateResponse{State: created.State}
	r.Update(ctx, resource.UpdateRequest{Plan: plan(m), State: created.State}, &updated)
	if updated.Diagnostics.HasError() || !saved.Configuration.DeleteEntraOnDestroy {
		t.Fatal(updated.Diagnostics)
	}
	imported := resource.ImportStateResponse{State: empty()}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "17"}, &imported)
	if imported.Diagnostics.HasError() {
		t.Fatal(imported.Diagnostics)
	}
	for _, denied := range []int{403, 409, 502} {
		status = denied
		read := resource.ReadResponse{State: imported.State}
		r.Read(ctx, resource.ReadRequest{State: imported.State}, &read)
		if !read.Diagnostics.HasError() || read.State.Raw.IsNull() {
			t.Fatalf("HTTP %d lost state", denied)
		}
	}
	status = 200
	confirmed := saved
	for _, corrupt := range []func(){
		func() { saved.ResourceID = "00000000-0000-0000-0000-000000000099" },
		func() { saved.GroupID = 0 },
		func() { saved.Configuration.Name = "" },
	} {
		corrupt()
		read := resource.ReadResponse{State: imported.State}
		r.Read(ctx, resource.ReadRequest{State: imported.State}, &read)
		if !read.Diagnostics.HasError() || !read.State.Raw.Equal(imported.State.Raw) {
			t.Fatal("malformed success response changed Terraform state")
		}
		saved = confirmed
	}
	deleted := resource.DeleteResponse{State: updated.State}
	r.Delete(ctx, resource.DeleteRequest{State: updated.State}, &deleted)
	if deleted.Diagnostics.HasError() || deletes != 1 {
		t.Fatal(deleted.Diagnostics)
	}
	status = 404
	gone := resource.ReadResponse{State: imported.State}
	r.Read(ctx, resource.ReadRequest{State: imported.State}, &gone)
	if gone.Diagnostics.HasError() || !gone.State.Raw.IsNull() {
		t.Fatal(gone.Diagnostics)
	}
}

func TestAZGroupAmbiguousCreationPreservesPendingIdentity(t *testing.T) {
	ctx := context.Background()
	var id string
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "POST" {
			writes++
			id = req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
			w.WriteHeader(400)
			return
		}
		_ = json.NewEncoder(w).Encode(azclient.Group{ResourceID: id, GroupID: 9, DisplayName: "Team", ActualSource: "Entra", Status: "Pending", Configuration: azclient.GroupConfiguration{Name: "Team", Source: "Auto", IsActive: true}})
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "ignored", "token", nil, time.Second)
	r := &azgroupResource{client: api}
	sr := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	p := tfsdk.Plan{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
	if d := p.Set(ctx, groupTestModel()); d.HasError() {
		t.Fatal(d)
	}
	resp := resource.CreateResponse{State: tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}}
	r.Create(ctx, resource.CreateRequest{Plan: p}, &resp)
	if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() || writes != 1 {
		t.Fatal("pending resource lost", resp.Diagnostics)
	}
	var m azgroupModel
	if d := resp.State.Get(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	if m.ID.ValueString() != id || m.Status.ValueString() != "Pending" {
		t.Fatal(m)
	}
}

func TestGroupMembershipAllKindsAndRoleChanges(t *testing.T) {
	for kind, apiKind := range membershipKinds {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			target := "11111111-2222-4333-8444-555555555555"
			if kind == "integration" {
				target = "8"
			}
			status := 200
			var role *string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if req.URL.Path != "/api/terraform/v1/group-memberships/7/"+apiKind+"/"+target {
					t.Error(req.URL.Path)
				}
				if status != 200 {
					w.WriteHeader(status)
					return
				}
				if req.Method == "DELETE" {
					w.WriteHeader(204)
					return
				}
				if req.Method == "PUT" {
					var body struct {
						Role *string `json:"role"`
					}
					_ = json.NewDecoder(req.Body).Decode(&body)
					role = body.Role
				}
				_ = json.NewEncoder(w).Encode(azclient.GroupMembership{GroupID: 7, TargetID: target, Role: role})
			}))
			defer server.Close()
			api, _ := azclient.New(server.URL, "ignored", "token", nil, time.Second)
			r := &azgroupMembershipResource{client: api}
			sr := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &sr)
			if d := sr.Schema.ValidateImplementation(ctx); d.HasError() {
				t.Fatal(d)
			}
			m := azgroupMembershipModel{ID: types.StringUnknown(), GroupID: types.Int64Value(7), Kind: types.StringValue(kind), TargetID: types.StringValue(target), Role: types.StringNull()}
			if kind == "user" {
				m.Role = types.StringValue("Owner")
			}
			p := tfsdk.Plan{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
			if d := p.Set(ctx, m); d.HasError() {
				t.Fatal(d)
			}
			created := resource.CreateResponse{State: tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}}
			r.Create(ctx, resource.CreateRequest{Plan: p}, &created)
			if created.Diagnostics.HasError() {
				t.Fatal(created.Diagnostics)
			}
			if kind == "user" && (role == nil || *role != "Admin") {
				t.Fatal("owner not mapped")
			}
			status = 403
			read := resource.ReadResponse{State: created.State}
			r.Read(ctx, resource.ReadRequest{State: created.State}, &read)
			if !read.Diagnostics.HasError() || read.State.Raw.IsNull() {
				t.Fatal("authorization loss removed membership state")
			}
			status = 200
			deleted := resource.DeleteResponse{State: created.State}
			r.Delete(ctx, resource.DeleteRequest{State: created.State}, &deleted)
			if deleted.Diagnostics.HasError() {
				t.Fatal(deleted.Diagnostics)
			}
			status = 404
			gone := resource.ReadResponse{State: created.State}
			r.Read(ctx, resource.ReadRequest{State: created.State}, &gone)
			if gone.Diagnostics.HasError() || !gone.State.Raw.IsNull() {
				t.Fatal(gone.Diagnostics)
			}
		})
	}
}

func TestGroupValidationRejectsUnsupportedConfiguration(t *testing.T) {
	for _, name := range []string{"", " Team", "Team\n", strings.Repeat("a", 201)} {
		m := groupTestModel()
		m.Name = types.StringValue(name)
		if !validateAZGroup(m).HasError() {
			t.Fatal(name)
		}
	}
	m := groupTestModel()
	m.Source = types.StringValue("unknown")
	if !validateAZGroup(m).HasError() {
		t.Fatal("invalid source accepted")
	}
	member := azgroupMembershipModel{GroupID: types.Int64Value(2), Kind: types.StringValue("integration"), TargetID: types.StringValue("3"), Role: types.StringValue("Owner")}
	if !validateGroupMembership(member).HasError() {
		t.Fatal("integration role accepted")
	}
	member.Role = types.StringNull()
	member.TargetID = types.StringValue("../4")
	if !validateGroupMembership(member).HasError() {
		t.Fatal("invalid path accepted")
	}
}
