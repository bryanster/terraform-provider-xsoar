package xsoar

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type resourceIntegrationInstanceType struct{}

func getIntegrationsFromAPIResponse(ctx context.Context, integration map[string]any, secretConfigs map[string]any, diag diag.Diagnostics) (string, error) {
	integrationConfigs := make(map[string]any)
	if integration["data"] == nil {
		integrationConfigs = map[string]any{}
	} else {
		var integrationConfig map[string]interface{}
		switch reflect.TypeOf(integration["data"]).Kind() {
		case reflect.Slice:
			s := reflect.ValueOf(integration["data"])
			for i := 0; i < s.Len(); i++ {
				integrationConfig = s.Index(i).Interface().(map[string]interface{})
				nameconf, ok := integrationConfig["name"].(string)
				if ok {
					_, ok := secretConfigs[nameconf]
					if !ok {
						integrationConfigs[nameconf] = integrationConfig["value"]
					}
				} else {
					break
				}
			}
		}
	}
	integrationConfigsJson, err := json.Marshal(integrationConfigs)
	if err != nil {
		diag.AddError(
			"Error parsing incoming integration instance",
			"Could not re-marshal incoming integration instance config json: "+err.Error(),
		)
		return "", err
	}

	return string(integrationConfigsJson), nil
}

// GetSchema Resource schema
func (r resourceIntegrationInstanceType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	var planModifiers []tfsdk.AttributePlanModifier
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"name": {
				Type:     types.StringType,
				Required: true,
			},
			"id": {
				Type:     types.StringType,
				Computed: true,
				Optional: false,
			},
			"integration_name": {
				Type:          types.StringType,
				Required:      true,
				PlanModifiers: append(planModifiers, tfsdk.RequiresReplace()),
			},
			"enabled": {
				Type:     types.BoolType,
				Optional: true,
				Computed: true,
			},
			"config_json": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"secret_config_json": {
				Type:     types.StringType,
				Optional: true,
				Computed: true,
			},
			"propagation_labels": {
				Type:     types.SetType{ElemType: types.StringType},
				Computed: true,
				Optional: true,
			},
			"account": {
				Type:          types.StringType,
				Optional:      true,
				PlanModifiers: append(planModifiers, tfsdk.RequiresReplace()),
			},
			"incoming_mapper_id": {
				Type:     types.StringType,
				Required: true,
			},
			// aka classifier
			"mapping_id": {
				Type:     types.StringType,
				Required: true,
			},
			"engine_id": {
				Type:     types.StringType,
				Required: true,
			},
		},
	}, nil
}

// NewResource instance
func (r resourceIntegrationInstanceType) NewResource(_ context.Context, p tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	return resourceIntegrationInstance{
		p: *(p.(*provider)),
	}, nil
}

type resourceIntegrationInstance struct {
	p provider
}

// Create a new resource
func (r resourceIntegrationInstance) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	if !r.p.configured {
		resp.Diagnostics.AddError(
			"Provider not configured",
			"The provider hasn't been configured before apply, likely because it depends on an unknown value from another resource. This leads to weird stuff happening, so we'd prefer if you didn't do that. Thanks!",
		)
		return
	}

	// Retrieve values from plan
	var plan IntegrationInstance
	var tf_config IntegrationInstance
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	diags = req.Config.Get(ctx, &tf_config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create
	// list integrations
	integrations, _, err := r.p.client.DefaultApi.ListIntegrations(ctx).Execute()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing integration",
			"Could not list integrations: "+err.Error(),
		)
		return
	}
	var moduleConfiguration []interface{}
	var moduleInstance = make(map[string]interface{})
	configurations := integrations["configurations"].([]interface{})
	for _, configuration := range configurations {
		config := configuration.(map[string]interface{})
		if config["name"].(string) == plan.IntegrationName.Value {
			moduleConfiguration = config["configuration"].([]interface{})
			moduleInstance["brand"] = config["name"].(string)
			moduleInstance["canSample"] = config["canGetSamples"].(bool)
			moduleInstance["category"] = config["category"].(string)
			moduleInstance["configuration"] = configuration
			moduleInstance["data"] = make([]map[string]interface{}, 0)
			moduleInstance["defaultIgnore"] = false
			var Enabled string
			if plan.Enabled.Null {
				Enabled = "true"
			} else {
				Enabled = strconv.FormatBool(plan.Enabled.Value)
			}
			moduleInstance["enabled"] = Enabled
			// todo: add this as a config option

			var EngineId string
			if ok := plan.EngineId.Value; ok != "" {
				EngineId = plan.EngineId.Value
			} else {
				EngineId = ""
			}
			moduleInstance["engine"] = EngineId

			// moduleInstance["engine"] = ""
			//moduleInstance["engineGroup"] = ""
			//moduleInstance["id"] = ""
			var IncomingMapperId string
			if ok := plan.IncomingMapperId.Value; ok != "" {
				IncomingMapperId = plan.IncomingMapperId.Value
			} else {
				IncomingMapperId = ""
			}
			moduleInstance["incomingMapperId"] = IncomingMapperId
			var MappingId string
			if ok := plan.MappingId.Value; ok != "" {
				MappingId = plan.MappingId.Value
			} else {
				MappingId = ""
			}
			moduleInstance["mappingId"] = MappingId
			//moduleInstance["integrationLogLevel"] = ""
			// todo: add this as a config option (byoi)
			var isIntegrationScript bool
			if val, ok := config["integrationScript"]; ok && val != nil {
				isIntegrationScript = true
			}
			moduleInstance["isIntegrationScript"] = isIntegrationScript
			//moduleInstance["isLongRunning"] = false
			moduleInstance["name"] = plan.Name.Value
			//moduleInstance["outgoingMapperId"] = ""
			//moduleInstance["passwordProtected"] = false
			var propLabels []string
			plan.PropagationLabels.ElementsAs(ctx, &propLabels, false)
			moduleInstance["propagationLabels"] = propLabels
			//moduleInstance["resetContext"] = false
			moduleInstance["version"] = -1
			break
		}
	}
	var configs map[string]any
	if plan.ConfigJson.Null {
		configs = map[string]any{}
	} else {
		err = json.Unmarshal([]byte(plan.ConfigJson.Value), &configs)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating integration instance",
				"Could not parse integration instance config json: "+err.Error(),
			)
			return
		}
	}
	var secretConfigs map[string]any
	var secretConfigJson string
	if plan.SecretConfigJson.Null || plan.SecretConfigJson.Value == "" {
		secretConfigJson = ""
		secretConfigs = map[string]any{}
	} else {
		secretConfigJson = plan.SecretConfigJson.Value
		err = json.Unmarshal([]byte(plan.SecretConfigJson.Value), &secretConfigs)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating integration instance",
				"Could not parse integration instance secret config json: "+err.Error()+"\n\""+plan.SecretConfigJson.Value+"\"",
			)
			return
		}
	}
	for key, element := range secretConfigs {
		if _, ok := configs[key]; ok {
			resp.Diagnostics.AddError(
				"Error creating integration instance",
				"Key: '"+key+"' exists in 'secret_config_json' and 'config_json'. Please choose 1.",
			)
			return
		}
		configs[key] = element
	}
	for _, parameter := range moduleConfiguration {
		param := parameter.(map[string]interface{})
		param["hasvalue"] = false
		for configName, configValue := range configs {
			if param["display"].(string) == configName || param["name"].(string) == configName {
				param["value"] = configValue
				param["hasvalue"] = true
				break
			}
		}
		moduleInstance["data"] = append(moduleInstance["data"].([]map[string]interface{}), param)
	}

	var integration map[string]interface{}
	var httpResponse *http.Response
	if plan.Account.Null || len(plan.Account.Value) == 0 {
		integration, httpResponse, err = r.p.client.DefaultApi.CreateUpdateIntegrationInstance(ctx).CreateIntegrationRequest(moduleInstance).Execute()
	} else {
		integration, httpResponse, err = r.p.client.DefaultApi.CreateUpdateIntegrationInstanceAccount(ctx, "acc_"+plan.Account.Value).CreateIntegrationRequest(moduleInstance).Execute()
	}
	if err != nil {
		if httpResponse != nil {
			body, _ := io.ReadAll(httpResponse.Body)
			payload, _ := io.ReadAll(httpResponse.Request.Body)
			log.Printf("code: %d status: %s headers: %s body: %s payload: %s\n", httpResponse.StatusCode, httpResponse.Status, httpResponse.Header, string(body), string(payload))
		}
		resp.Diagnostics.AddError(
			"Error creating integration instance",
			"Could not create integration instance: "+err.Error(),
		)
		return
	}

	var propagationLabels []attr.Value
	if integration["propagationLabels"] == nil {
		propagationLabels = []attr.Value{}
	} else {
		for _, prop := range integration["propagationLabels"].([]interface{}) {
			propagationLabels = append(propagationLabels, types.String{
				Unknown: false,
				Null:    false,
				Value:   prop.(string),
			})
		}
	}

	integrationConfigsJson, err := getIntegrationsFromAPIResponse(ctx, integration, secretConfigs, resp.Diagnostics)
	if err != nil {
		return
	}

	// Map response body to resource schema attribute
	result := IntegrationInstance{
		Name:              types.String{Value: integration["name"].(string)},
		Id:                types.String{Value: integration["id"].(string)},
		IntegrationName:   types.String{Value: integration["brand"].(string)},
		Account:           plan.Account,
		PropagationLabels: types.Set{Elems: propagationLabels, ElemType: types.StringType},
		ConfigJson:        types.String{Value: integrationConfigsJson},
		SecretConfigJson:  types.String{Value: secretConfigJson},
	}

	Enabled, err := strconv.ParseBool(integration["enabled"].(string))
	if err == nil {
		result.Enabled = types.Bool{Value: Enabled}
	} else {
		result.Enabled = types.Bool{Null: true}
	}

	IncomingMapperId, ok := integration["incomingMapperId"].(string)
	if ok {
		result.IncomingMapperId = types.String{Value: IncomingMapperId}
	} else {
		result.IncomingMapperId = types.String{Null: true}
	}

	MappingId, ok := integration["mappingId"].(string)
	if ok {
		result.MappingId = types.String{Value: MappingId}
	} else {
		result.MappingId = types.String{Null: true}
	}

	EngineId, ok := integration["engine"].(string)
	if ok {
		result.EngineId = types.String{Value: EngineId}
	} else {
		result.EngineId = types.String{Null: true}
	}

	// Generate resource state struct
	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read resource information
func (r resourceIntegrationInstance) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	// Get current state
	var state IntegrationInstance
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get resource from API
	var integration map[string]interface{}
	var httpResponse *http.Response
	var err error
	if state.Account.Null || len(state.Account.Value) == 0 {
		integration, httpResponse, err = r.p.client.DefaultApi.GetIntegrationInstance(ctx).SetIdentifier(state.Id.Value).Execute()
	} else {
		var account map[string]interface{}
		account, httpResponse, err = r.p.client.DefaultApi.GetAccount(ctx, "acc_"+state.Account.Value).Execute()
		if err != nil {
			log.Println(err.Error())
			if httpResponse != nil {
				body, _ := io.ReadAll(httpResponse.Body)
				payload, _ := io.ReadAll(httpResponse.Request.Body)
				log.Printf("code: %d status: %s headers: %s body: %s payload: %s\n", httpResponse.StatusCode, httpResponse.Status, httpResponse.Header, string(body), string(payload))
			}
			resp.Diagnostics.AddError(
				"Error getting integration instance",
				"Could not verify account existence: "+err.Error(),
			)
			return
		}
		if account == nil {
			resp.State.RemoveResource(ctx)
			return
		}
		integration, httpResponse, err = r.p.client.DefaultApi.GetIntegrationInstanceAccount(ctx, "acc_"+state.Account.Value).SetIdentifier(state.Id.Value).Execute()
	}
	if err != nil {
		log.Println(err.Error())
		if httpResponse != nil {
			body, _ := io.ReadAll(httpResponse.Body)
			payload, _ := io.ReadAll(httpResponse.Request.Body)
			log.Printf("code: %d status: %s headers: %s body: %s payload: %s\n", httpResponse.StatusCode, httpResponse.Status, httpResponse.Header, string(body), string(payload))
		}
		resp.Diagnostics.AddError(
			"Error getting integration instance",
			"Could not get integration instance: "+err.Error(),
		)
		return
	}

	if integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var propagationLabels []attr.Value
	if integration["propagationLabels"] == nil {
		propagationLabels = []attr.Value{}
	} else {
		for _, prop := range integration["propagationLabels"].([]interface{}) {
			propagationLabels = append(propagationLabels, types.String{
				Unknown: false,
				Null:    false,
				Value:   prop.(string),
			})
		}
	}

	var secretConfigs map[string]any
	var secretConfigJson string
	if state.SecretConfigJson.Null || state.SecretConfigJson.Value == "" {
		secretConfigJson = ""
		secretConfigs = map[string]any{}
	} else {
		secretConfigJson = state.SecretConfigJson.Value
		err = json.Unmarshal([]byte(state.SecretConfigJson.Value), &secretConfigs)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating integration instance",
				"Could not parse integration instance secret config json: "+err.Error()+"\n\""+state.SecretConfigJson.Value+"\"",
			)
			return
		}
	}
	integrationConfigsJson, err := getIntegrationsFromAPIResponse(ctx, integration, secretConfigs, resp.Diagnostics)
	if err != nil {
		return
	}

	// Map response body to resource schema attribute
	result := IntegrationInstance{
		Name:              types.String{Value: integration["name"].(string)},
		Id:                types.String{Value: integration["id"].(string)},
		IntegrationName:   types.String{Value: integration["brand"].(string)},
		Account:           state.Account,
		PropagationLabels: types.Set{Elems: propagationLabels, ElemType: types.StringType},
		ConfigJson:        types.String{Value: integrationConfigsJson},
		SecretConfigJson:  types.String{Value: secretConfigJson},
	}

	Enabled, err := strconv.ParseBool(integration["enabled"].(string))
	if err == nil {
		result.Enabled = types.Bool{Value: Enabled}
	} else {
		result.Enabled = types.Bool{Null: true}
	}

	IncomingMapperId, ok := integration["incomingMapperId"].(string)
	if ok {
		result.IncomingMapperId = types.String{Value: IncomingMapperId}
	} else {
		result.IncomingMapperId = types.String{Null: true}
	}

	MappingId, ok := integration["mappingId"].(string)
	if ok {
		result.MappingId = types.String{Value: MappingId}
	} else {
		result.MappingId = types.String{Null: true}
	}

	EngineId, ok := integration["engine"].(string)
	if ok {
		result.EngineId = types.String{Value: EngineId}
	} else {
		result.EngineId = types.String{Null: true}
	}

	// Generate resource state struct
	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update resource
func (r resourceIntegrationInstance) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
	// Get plan values
	var plan IntegrationInstance
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state
	var state IntegrationInstance
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build request
	// list integrations
	integrations, _, err := r.p.client.DefaultApi.ListIntegrations(ctx).Execute()
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing integration",
			"Could not list integrations: "+err.Error(),
		)
		return
	}
	var moduleConfiguration []interface{}
	var moduleInstance = make(map[string]interface{})
	configurations := integrations["configurations"].([]interface{})
	for _, configuration := range configurations {
		config := configuration.(map[string]interface{})
		if config["name"].(string) == plan.IntegrationName.Value {
			moduleConfiguration = config["configuration"].([]interface{})
			moduleInstance["brand"] = config["name"].(string)
			moduleInstance["canSample"] = config["canGetSamples"].(bool)
			moduleInstance["category"] = config["category"].(string)
			moduleInstance["configuration"] = configuration
			moduleInstance["data"] = make([]map[string]interface{}, 0)
			moduleInstance["defaultIgnore"] = false
			var Enabled string
			if plan.Enabled.Null {
				Enabled = "true"
			} else {
				Enabled = strconv.FormatBool(plan.Enabled.Value)
			}
			moduleInstance["enabled"] = Enabled
			// todo: add this as a config option

			var EngineId string
			if ok := plan.EngineId.Value; ok != "" {
				EngineId = plan.EngineId.Value
			} else {
				EngineId = ""
			}
			moduleInstance["engine"] = EngineId

			//moduleInstance["engine"] = ""
			//moduleInstance["engineGroup"] = ""
			moduleInstance["id"] = state.Id.Value
			var IncomingMapperId string
			if ok := plan.IncomingMapperId.Value; ok != "" {
				IncomingMapperId = plan.IncomingMapperId.Value
			} else {
				IncomingMapperId = ""
			}
			moduleInstance["incomingMapperId"] = IncomingMapperId
			var MappingId string
			if ok := plan.MappingId.Value; ok != "" {
				MappingId = plan.MappingId.Value
			} else {
				MappingId = ""
			}
			moduleInstance["mappingId"] = MappingId
			//moduleInstance["integrationLogLevel"] = ""
			// todo: add this as a config option (byoi)
			var isIntegrationScript bool
			if val, ok := config["integrationScript"]; ok && val != nil {
				isIntegrationScript = true
			}
			moduleInstance["isIntegrationScript"] = isIntegrationScript
			//moduleInstance["isLongRunning"] = false
			moduleInstance["name"] = plan.Name.Value
			//moduleInstance["outgoingMapperId"] = ""
			//moduleInstance["passwordProtected"] = false
			var propLabels []string
			plan.PropagationLabels.ElementsAs(ctx, &propLabels, false)
			moduleInstance["propagationLabels"] = propLabels
			//moduleInstance["resetContext"] = false
			moduleInstance["version"] = -1
			break
		}
	}

	var configs map[string]any
	if plan.ConfigJson.Null {
		configs = map[string]any{}
	} else {
		err = json.Unmarshal([]byte(plan.ConfigJson.Value), &configs)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating integration instance",
				"Could not parse integration instance config json: "+err.Error(),
			)
			return
		}
	}
	var secretConfigs map[string]any
	var secretConfigJson string
	if plan.SecretConfigJson.Null || plan.SecretConfigJson.Value == "" {
		secretConfigJson = ""
		secretConfigs = map[string]any{}
	} else {
		secretConfigJson = plan.SecretConfigJson.Value
		err = json.Unmarshal([]byte(plan.SecretConfigJson.Value), &secretConfigs)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating integration instance",
				"Could not parse integration instance secret config json: "+err.Error()+"\n\""+plan.SecretConfigJson.Value+"\"",
			)
			return
		}
	}
	for key, element := range secretConfigs {
		if _, ok := configs[key]; ok {
			resp.Diagnostics.AddError(
				"Error updating integration instance",
				"Key: '"+key+"' exists in 'secret_config_json' and 'config_json'. Please choose 1.",
			)
			return
		}
		configs[key] = element
	}
	for _, parameter := range moduleConfiguration {
		param := parameter.(map[string]interface{})
		param["hasvalue"] = false
		for configName, configValue := range configs {
			if param["display"].(string) == configName || param["name"].(string) == configName {
				param["value"] = configValue
				param["hasvalue"] = true
				break
			}
		}
		if !param["hasvalue"].(bool) {
			param["value"] = param["defaultValue"].(string)
		}
		moduleInstance["data"] = append(moduleInstance["data"].([]map[string]interface{}), param)
	}

	var integration map[string]interface{}
	var httpResponse *http.Response
	if state.Account.Null || len(state.Account.Value) == 0 {
		integration, httpResponse, err = r.p.client.DefaultApi.CreateUpdateIntegrationInstance(ctx).CreateIntegrationRequest(moduleInstance).Execute()
	} else {
		integration, httpResponse, err = r.p.client.DefaultApi.CreateUpdateIntegrationInstanceAccount(ctx, "acc_"+plan.Account.Value).CreateIntegrationRequest(moduleInstance).Execute()
	}
	if err != nil {
		if httpResponse != nil {
			body, _ := io.ReadAll(httpResponse.Body)
			payload, _ := io.ReadAll(httpResponse.Request.Body)
			log.Printf("code: %d status: %s headers: %s body: %s payload: %s\n", httpResponse.StatusCode, httpResponse.Status, httpResponse.Header, string(body), string(payload))
		}
		resp.Diagnostics.AddError(
			"Error updating integration instance",
			"Could not update integration instance: "+err.Error(),
		)
		return
	}

	var propagationLabels []attr.Value
	if integration["propagationLabels"] == nil {
		propagationLabels = []attr.Value{}
	} else {
		for _, prop := range integration["propagationLabels"].([]interface{}) {
			propagationLabels = append(propagationLabels, types.String{
				Unknown: false,
				Null:    false,
				Value:   prop.(string),
			})
		}
	}

	integrationConfigsJson, err := getIntegrationsFromAPIResponse(ctx, integration, secretConfigs, resp.Diagnostics)
	if err != nil {
		return
	}

	// Map response body to resource schema attribute
	result := IntegrationInstance{
		Name:              types.String{Value: integration["name"].(string)},
		Id:                types.String{Value: integration["id"].(string)},
		IntegrationName:   types.String{Value: integration["brand"].(string)},
		Account:           plan.Account,
		PropagationLabels: types.Set{Elems: propagationLabels, ElemType: types.StringType},
		ConfigJson:        types.String{Value: integrationConfigsJson},
		SecretConfigJson:  types.String{Value: secretConfigJson},
	}

	Enabled, err := strconv.ParseBool(integration["enabled"].(string))
	if err == nil {
		result.Enabled = types.Bool{Value: Enabled}
	} else {
		result.Enabled = types.Bool{Null: true}
	}

	IncomingMapperId, ok := integration["incomingMapperId"].(string)
	if ok {
		result.IncomingMapperId = types.String{Value: IncomingMapperId}
	} else {
		result.IncomingMapperId = types.String{Null: true}
	}

	MappingId, ok := integration["mappingId"].(string)
	if ok {
		result.MappingId = types.String{Value: MappingId}
	} else {
		result.MappingId = types.String{Null: true}
	}

	EngineId, ok := integration["engine"].(string)
	if ok {
		result.EngineId = types.String{Value: EngineId}
	} else {
		result.EngineId = types.String{Null: true}
	}

	// Set state
	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete resource
func (r resourceIntegrationInstance) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	// Get state
	var state IntegrationInstance
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete
	var err error
	if state.Account.Null || len(state.Account.Value) == 0 {
		_, err = r.p.client.DefaultApi.DeleteIntegrationInstance(ctx, state.Id.Value).Execute()
	} else {
		_, err = r.p.client.DefaultApi.DeleteIntegrationInstanceAccount(ctx, state.Id.Value, "acc_"+state.Account.Value).Execute()
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting integration instance",
			"Could not delete integration instance: "+err.Error(),
		)
		return
	}

	// Remove resource from state
	resp.State.RemoveResource(ctx)
}

func (r resourceIntegrationInstance) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	var diags diag.Diagnostics
	accname := strings.Split(req.ID, ".")
	var acc, name string
	var integration map[string]interface{}
	var err error
	if len(accname) == 1 {
		name = req.ID
		integration, _, err = r.p.client.DefaultApi.GetIntegrationInstance(ctx).SetIdentifier(name).Execute()
	} else {
		acc, name = accname[0], accname[1]
		integration, _, err = r.p.client.DefaultApi.GetIntegrationInstanceAccount(ctx, "acc_"+acc).SetIdentifier(name).Execute()
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error getting integration instance",
			"Could not get integration instance: "+err.Error(),
		)
		return
	}
	if integration == nil {
		resp.Diagnostics.AddError(
			"Integration instance not found",
			"Could not find integration instance: "+name,
		)
		return
	}

	var propagationLabels []attr.Value
	if integration["propagationLabels"] == nil {
		propagationLabels = []attr.Value{}
	} else {
		for _, prop := range integration["propagationLabels"].([]interface{}) {
			propagationLabels = append(propagationLabels, types.String{
				Unknown: false,
				Null:    false,
				Value:   prop.(string),
			})
		}
	}

	integrationConfigsJson, err := getIntegrationsFromAPIResponse(ctx, integration, map[string]any{}, resp.Diagnostics)
	if err != nil {
		return
	}

	// Map response body to resource schema attribute
	result := IntegrationInstance{
		Name:              types.String{Value: integration["name"].(string)},
		Id:                types.String{Value: integration["id"].(string)},
		IntegrationName:   types.String{Value: integration["brand"].(string)},
		PropagationLabels: types.Set{Elems: propagationLabels, ElemType: types.StringType},
		ConfigJson:        types.String{Value: integrationConfigsJson},
		SecretConfigJson:  types.String{Value: "{}"},
	}

	Enabled, err := strconv.ParseBool(integration["enabled"].(string))
	if err == nil {
		result.Enabled = types.Bool{Value: Enabled}
	} else {
		result.Enabled = types.Bool{Null: true}
	}

	IncomingMapperId, ok := integration["incomingMapperId"].(string)
	if ok {
		result.IncomingMapperId = types.String{Value: IncomingMapperId}
	} else {
		result.IncomingMapperId = types.String{Null: true}
	}

	MappingId, ok := integration["mappingId"].(string)
	if ok {
		result.MappingId = types.String{Value: MappingId}
	} else {
		result.MappingId = types.String{Null: true}
	}

	EngineId, ok := integration["engine"].(string)
	if ok {
		result.EngineId = types.String{Value: EngineId}
	} else {
		result.EngineId = types.String{Null: true}
	}

	if acc != "" {
		result.Account = types.String{Value: acc}
	} else {
		result.Account = types.String{Null: true}
	}

	// Generate resource state struct
	diags = resp.State.Set(ctx, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
