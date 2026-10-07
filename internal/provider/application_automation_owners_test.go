package provider

import (
	"context"
	"testing"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAutomationOwnershipDoesNotCausePerpetualDrift(t *testing.T) {
	ctx := context.Background()
	const machine = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	const human = "11111111-2222-4333-8444-555555555555"
	const other = "22222222-2222-4333-8444-555555555555"
	for _, explicit := range []bool{false, true} {
		owners := []string{human}
		if explicit {
			owners = append(owners, machine)
		}
		configured, d := types.SetValueFrom(ctx, types.StringType, owners)
		if d.HasError() {
			t.Fatal(d)
		}
		model := applicationResourceModel{OwnerObjectIDs: configured}
		observed := &azclient.Application{OwnerObjectIDs: []string{human, machine}, AutomationOwnerObjectIDs: []string{machine}}
		// The same mapper handles apply, refresh, and both application resources.
		for i := 0; i < 3; i++ {
			var diagnostics diag.Diagnostics
			mapApplicationToModel(ctx, observed, &model, &diagnostics)
			if diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			if !model.OwnerObjectIDs.Equal(configured) {
				t.Fatalf("explicit=%v: unexpected owners %s", explicit, model.OwnerObjectIDs)
			}
		}
		observed.OwnerObjectIDs = append(observed.OwnerObjectIDs, other)
		var diagnostics diag.Diagnostics
		mapApplicationToModel(ctx, observed, &model, &diagnostics)
		if model.OwnerObjectIDs.Equal(configured) {
			t.Fatal("ordinary owner drift was hidden")
		}
	}
}

func TestAutomationOwnerFilteringPreservesMissingOwnersAndLegacyAPIs(t *testing.T) {
	ctx := context.Background()
	for _, desired := range []types.Set{types.SetNull(types.StringType), types.SetUnknown(types.StringType), types.SetValueMust(types.StringType, nil)} {
		result := managedOwnerObjectIDs(&azclient.Application{OwnerObjectIDs: []string{"human", "machine"}, AutomationOwnerObjectIDs: []string{"machine"}}, desired)
		if len(result) != 1 || result[0] != "human" {
			t.Fatalf("unexpected owners: %v", result)
		}
	}
	configured, _ := types.SetValueFrom(ctx, types.StringType, []string{"human", "machine"})
	missing := managedOwnerObjectIDs(&azclient.Application{OwnerObjectIDs: []string{"human"}, AutomationOwnerObjectIDs: []string{"machine"}}, configured)
	if len(missing) != 1 {
		t.Fatal("missing explicit automation owner must remain drift")
	}
	legacy := managedOwnerObjectIDs(&azclient.Application{OwnerObjectIDs: []string{"human", "machine"}}, types.SetNull(types.StringType))
	if len(legacy) != 2 {
		t.Fatal("legacy API must preserve existing semantics")
	}
}
