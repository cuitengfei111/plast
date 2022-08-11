package lts

import (
	"context"
	"log"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	lts "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/lts/v2/model"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/common"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/config"
	"github.com/huaweicloud/terraform-provider-huaweicloud/huaweicloud/utils/fmtp"
)

func ResourceAlarmRule() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAomMappingRuleCreate,
		ReadContext:   resourceAomMappingRuleRead,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(5 * time.Minute),
			Update: schema.DefaultTimeout(5 * time.Minute),
			Delete: schema.DefaultTimeout(5 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
			"isBatch": {
				Type: schema.TypeBool,
				Required: true,
				ForceNew: true,
			},
			"rule_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"cluster_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"cluster_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"container_name": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"deployments": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"files": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"file_name": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"log_stream_info": {
							Type:     schema.TypeList,
							Required: true,
							ForceNew: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"target_log_group_id": {
										Type:     schema.TypeString,
										Required: true,
										ForceNew: true,
									},
									"target_log_group_name": {
										Type:     schema.TypeString,
										Required: true,
										ForceNew: true,
									},
									"target_log_stream_id": {
										Type:     schema.TypeString,
										Required: true,
										ForceNew: true,
									},
									"target_log_stream_name": {
										Type:     schema.TypeString,
										Required: true,
										ForceNew: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func buildLogStreamOpts(rawRule interface{}) *lts.AomMappingLogStreamInfo {
	s := rawRule.(map[string]interface{})
	rst := lts.AomMappingLogStreamInfo{
		TargetLogGroupId:    s["target_log_group_id"].(string),
		TargetLogGroupName:  s["target_log_group_name"].(string),
		TargetLogStreamId:   s["target_log_stream_id"].(string),
		TargetLogStreamName: s["target_log_stream_name"].(string),
	}
	return &rst
}

func buildFileOpts(rawRules []interface{}) []lts.AomMappingfilesInfo {
	file := make([]lts.AomMappingfilesInfo, len(rawRules))
	for i, v := range rawRules {
		rawRule := v.(map[string]interface{})
		file[i].FileName = rawRule["file_name"].(string)
		file[i].LogStreamInfo = buildLogStreamOpts(rawRule["log_stream_info"])
	}
	return file
}

func resourceAomMappingRuleCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	config := meta.(*config.Config)
	client, err := config.HcLtsV2Client(config.GetRegion(d))
	if err != nil {
		return fmtp.DiagErrorf("error creating LTS client: %s", err)
	}
	createOpts := lts.AomMappingRequestInfo{
		ProjectId: config.HwClient.ProjectID,
		RuleName:  d.Get("rule_name").(string),
		RuleInfo: &lts.AomMappingRuleInfo{
			ClusterId:     d.Get("cluster_id").(string),
			ClusterName:   d.Get("cluster_name").(string),
			Namespace:     d.Get("name_space").(string),
			ContainerName: d.Get("container_name").(*string),
			Files:         buildFileOpts(d.Get("files").([]interface{})),
		},
	}

	log.Printf("[DEBUG] Create %s Options: %#v", createOpts.RuleName, createOpts)

	createReq := lts.CreateAomMappingRulesRequest{
		IsBatch: d.Get("isBatch").(bool),
		Body: &createOpts,
	}
	response, err := client.CreateAomMappingRules(&createReq)
	if err != nil || len(*response.Body) == 0 {
		return diag.Errorf("error creating AOM mapping rule %s: %s", createOpts.RuleName, err)
	}
	id := *response.Body
	Id := id[0].RuleId

	d.SetId(Id)

	return resourceAomMappingRuleRead(ctx, d, meta)
}

func resourceAomMappingRuleRead(_ context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	config := meta.(*config.Config)
	client, err := config.HcLtsV2Client(config.GetRegion(d))
	if err != nil {
		return fmtp.DiagErrorf("error creating AOM client: %s", err)
	}

	response, err := client.ShowAomMappingRule(&lts.ShowAomMappingRuleRequest{RuleId: d.Id()})
	if err != nil {
		return common.CheckDeletedDiag(d, err, "error retrieving AOM Mapping rule")
	}

	allRules := *response.Body
	if len(allRules) != 1 {
		return diag.Errorf("error retrieving AOM Mapping rule %s", d.Id())
	}
	rule := allRules[0]
	log.Printf("[DEBUG] Retrieved AOM Mapping rule %s: %#v", d.Id(), rule)

	mErr := multierror.Append(nil,
		d.Set("region", config.GetRegion(d)),
		d.Set("project_id", config.HwClient.ProjectID),
		d.Set("rule_name", rule.RuleName),
		d.Set("cluster_id", rule.RuleInfo.ClusterId),
		d.Set("cluster_name", rule.RuleInfo.ClusterName),
		d.Set("name_space", rule.RuleInfo.Namespace),
		d.Set("container_name", rule.RuleInfo.ContainerName),
		d.Set("files", flattenFilesAomMappingRule(rule.RuleInfo.Files)),
	)

	if err := mErr.ErrorOrNil(); err != nil {
		return fmtp.DiagErrorf("error setting AOM alarm rule fields: %w", err)
	}

	return nil
}

func flattenFilesAomMappingRule(rule []lts.AomMappingfilesInfo) []map[string]interface{} {
	var aommappingfilesinfos []map[string]interface{}
	for _, fileObject := range rule {
		aommappingfilesinfo := make(map[string]interface{})
		aommappingfilesinfo["file_name"] = fileObject.FileName
		aommappingfilesinfo["log_stream_info"] = fileObject.LogStreamInfo
		aommappingfilesinfos = append(aommappingfilesinfos, aommappingfilesinfo)
	}
	return aommappingfilesinfos
}
