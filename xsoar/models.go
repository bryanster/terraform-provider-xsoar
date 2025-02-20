package xsoar

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Account -
type Account struct {
	Name              types.String `tfsdk:"name"`
	Id                types.String `tfsdk:"id"`
	HostGroupName     types.String `tfsdk:"host_group_name"`
	HostGroupId       types.String `tfsdk:"host_group_id"`
	AccountRoles      types.Set    `tfsdk:"account_roles"`
	PropagationLabels types.Set    `tfsdk:"propagation_labels"`
	Timeout           types.Int64  `tfsdk:"timeout"`
	Concurrency       types.Int64  `tfsdk:"concurrency_limit"`
}

// Accounts -
type Accounts struct {
	Accounts types.Set `tfsdk:"accounts"`
}

// HAGroup -
type HAGroup struct {
	Name               types.String `tfsdk:"name"`
	Id                 types.String `tfsdk:"id"`
	ElasticsearchUrl   types.String `tfsdk:"elasticsearch_url"`
	ElasticIndexPrefix types.String `tfsdk:"elastic_index_prefix"`
	AccountIds         types.Set    `tfsdk:"account_ids"`
	HostIds            types.Set    `tfsdk:"host_ids"`
}

// HAGroups -
type HAGroups struct {
	Name        types.String `tfsdk:"name"`
	MaxAccounts types.Int64  `tfsdk:"max_accounts"`
	Groups      types.Set    `tfsdk:"groups"`
}

// Host -
type Host struct {
	Name                types.String `tfsdk:"name"`
	Id                  types.String `tfsdk:"id"`
	HAGroupName         types.String `tfsdk:"ha_group_name"`
	NFSMount            types.String `tfsdk:"nfs_mount"`
	ElasticsearchUrl    types.String `tfsdk:"elasticsearch_url"`
	ServerUrl           types.String `tfsdk:"server_url"`
	SSHUser             types.String `tfsdk:"ssh_user"`
	SSHKey              types.String `tfsdk:"ssh_key"`
	InstallationTimeout types.Int64  `tfsdk:"installation_timeout"`
	ExtraFlags          types.List   `tfsdk:"extra_flags"`
}

// IntegrationInstance -
type IntegrationInstance struct {
	Name              types.String `tfsdk:"name"`
	Id                types.String `tfsdk:"id"`
	IntegrationName   types.String `tfsdk:"integration_name"`
	Account           types.String `tfsdk:"account"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	PropagationLabels types.Set    `tfsdk:"propagation_labels"`
	ConfigJson        types.String `tfsdk:"config_json"`
	SecretConfigJson  types.String `tfsdk:"secret_config_json"`
	IncomingMapperId  types.String `tfsdk:"incoming_mapper_id"`
	MappingId         types.String `tfsdk:"mapping_id"`
	EngineId          types.String `tfsdk:"engine_id"`
}

// Classifier -
type Classifier struct {
	Name                types.String `tfsdk:"name"`
	Id                  types.String `tfsdk:"id"`
	DefaultIncidentType types.String `tfsdk:"default_incident_type"`
	KeyTypeMap          types.String `tfsdk:"key_type_map"`
	Transformer         types.String `tfsdk:"transformer"`
	PropagationLabels   types.Set    `tfsdk:"propagation_labels"`
	Account             types.String `tfsdk:"account"`
}

// Mapper -
type Mapper struct {
	Name              types.String `tfsdk:"name"`
	Id                types.String `tfsdk:"id"`
	Mapping           types.String `tfsdk:"mapping"`
	PropagationLabels types.Set    `tfsdk:"propagation_labels"`
	Account           types.String `tfsdk:"account"`
	Direction         types.String `tfsdk:"direction"`
}
