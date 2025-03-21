// Tencent Driver of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// This is Tencent Driver.
//
// by CB-Spider Team, 2022.09.

package resources

import (
	"encoding/json"
	"fmt"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	call "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/call-log"
	"github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/drivers/tencent/utils/tencent"
	idrv "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/interfaces"
	irs "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/interfaces/resources"

	tke "github.com/tencentcloud/tencentcloud-sdk-go-intl-en/tencentcloud/tke/v20180525"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
)

// calllogger
// 공통로거 만들기 이전까지 사용
// var once sync.Once
// var calllogger *logrus.Logger

// func init() {
// 	once.Do(func() {
// 		calllogger = call.GetLogger("HISCALL")
// 	})
// }

const (
	defaultContainerRuntime = "containerd"
)

type TencentClusterHandler struct {
	RegionInfo     idrv.RegionInfo
	CredentialInfo idrv.CredentialInfo
}

func (clusterHandler *TencentClusterHandler) CreateCluster(clusterReqInfo irs.ClusterInfo) (irs.ClusterInfo, error) {
	cblogger.Info("Tencent Cloud Driver: called CreateCluster()")
	callLogInfo := GetCallLogScheme(clusterHandler.RegionInfo, call.CLUSTER, "CreateCluster()", "CreateCluster()")

	start := call.Start()

	//
	// Validation
	//
	err := validateAtCreateCluster(clusterReqInfo)
	if err != nil {
		err = fmt.Errorf("Failed to Create Cluster :  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return irs.ClusterInfo{}, err
	}

	// 클러스터 생성 요청 변환
	request, err := getCreateClusterRequest(clusterHandler, clusterReqInfo)
	if err != nil {
		err := fmt.Errorf("Failed to Get Create Cluster Request :  %v", err)
		cblogger.Error(err)
		return irs.ClusterInfo{}, err
	}
	res, err := tencent.CreateCluster(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, request)
	callLogInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		err := fmt.Errorf("Failed to Create Cluster :  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return irs.ClusterInfo{}, err
	}
	calllogger.Info(call.String(callLogInfo))

	// NodeGroup 생성 정보가 있는경우 생성을 시도한다.
	// 현재는 생성 시도를 안한다. 생성하기로 결정되면 아래 주석을 풀어서 사용한다.
	// 이유:
	// - Cluster 생성이 완료되어야 NodeGroup 생성이 가능하다.
	// - Cluster 생성이 완료되려면 최소 10분 이상 걸린다.
	// - 성공할때까지 대기한 후에 생성을 시도해야 한다.
	// for _, node_group := range clusterReqInfo.NodeGroupList {
	// 	res, err := clusterHandler.AddNodeGroup(clusterReqInfo.IId, node_group)
	// 	if err != nil {
	// 		cblogger.Error(err)
	// 		return irs.ClusterInfo{}, err
	// 	}
	// }

	cluster_info, err := getClusterInfo(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, *res.Response.ClusterId)
	if err != nil {
		err := fmt.Errorf("Failed to Get ClusterInfo :  %v", err)
		cblogger.Error(err)
		return irs.ClusterInfo{}, err
	}

	return *cluster_info, nil
}

func (clusterHandler *TencentClusterHandler) ListCluster() ([]*irs.ClusterInfo, error) {
	cblogger.Info("Tencent Cloud Driver: called ListCluster()")
	callLogInfo := GetCallLogScheme(clusterHandler.RegionInfo, call.CLUSTER, "ListCluster()", "ListCluster()")

	start := call.Start()
	res, err := tencent.GetClusters(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region)
	callLogInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		err := fmt.Errorf("Failed to Get Clusters :  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return nil, err
	}
	calllogger.Info(call.String(callLogInfo))

	cluster_info_list := make([]*irs.ClusterInfo, *res.Response.TotalCount)
	for i, cluster := range res.Response.Clusters {
		cluster_info_list[i], err = getClusterInfo(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, *cluster.ClusterId)
		if err != nil {
			err := fmt.Errorf("Failed to Get ClusterInfo :  %v", err)
			cblogger.Error(err)
			return nil, err
		}
	}

	return cluster_info_list, nil
}

func (clusterHandler *TencentClusterHandler) GetCluster(clusterIID irs.IID) (irs.ClusterInfo, error) {
	cblogger.Info("Tencent Cloud Driver: called GetCluster()")
	callLogInfo := GetCallLogScheme(clusterHandler.RegionInfo, call.CLUSTER, clusterIID.NameId, "GetCluster()")

	start := call.Start()
	cluster_info, err := getClusterInfo(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId)
	callLogInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		err := fmt.Errorf("Failed to Get ClusterInfo :  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return irs.ClusterInfo{}, err
	}
	calllogger.Info(call.String(callLogInfo))

	return *cluster_info, nil
}

func (clusterHandler *TencentClusterHandler) DeleteCluster(clusterIID irs.IID) (bool, error) {
	cblogger.Info("Tencent Cloud Driver: called DeleteCluster()")
	callLogInfo := GetCallLogScheme(clusterHandler.RegionInfo, call.CLUSTER, clusterIID.NameId, "DeleteCluster()")

	start := call.Start()
	res, err := tencent.DeleteCluster(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId)
	callLogInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		err := fmt.Errorf("Failed to Delete Cluster :  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return false, err
	}
	cblogger.Info("DeleteCluster(): ", res)
	calllogger.Info(call.String(callLogInfo))

	return true, nil
}

func (clusterHandler *TencentClusterHandler) AddNodeGroup(clusterIID irs.IID, nodeGroupReqInfo irs.NodeGroupInfo) (irs.NodeGroupInfo, error) {
	cblogger.Info("Tencent Cloud Driver: called AddNodeGroup()")
	callLogInfo := GetCallLogScheme(clusterHandler.RegionInfo, call.CLUSTER, clusterIID.NameId, "AddNodeGroup()")

	start := call.Start()

	//
	// Validation
	//
	err := validateAtAddNodeGroup(clusterIID, nodeGroupReqInfo)
	if err != nil {
		err := fmt.Errorf("Failed to Add Node Group :  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return irs.NodeGroupInfo{}, err
	}

	// 노드 그룹 생성 요청 변환
	// get cluster info. to get security_group_id
	request, err := getNodeGroupRequest(clusterHandler, clusterIID.SystemId, nodeGroupReqInfo)
	if err != nil {
		err := fmt.Errorf("Failed to Get Node Group Request :  %v", err)
		cblogger.Error(err)
		return irs.NodeGroupInfo{}, err
	}
	response, err := tencent.CreateNodeGroup(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, request)
	callLogInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		err := fmt.Errorf("Failed to Add Node Group :  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return irs.NodeGroupInfo{}, err
	}
	calllogger.Info(call.String(callLogInfo))

	node_group_info, err := getNodeGroupInfo(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId, *response.Response.NodePoolId)
	if err != nil {
		err := fmt.Errorf("Failed to Get Node Group Info :  %v", err)
		cblogger.Error(err)
		return irs.NodeGroupInfo{}, err
	}

	return *node_group_info, nil
}

// func (clusterHandler *TencentClusterHandler) ListNodeGroup(clusterIID irs.IID) ([]*irs.NodeGroupInfo, error) {
// 	cblogger.Info("Tencent Cloud Driver: called ListNodeGroup()")
// 	callLogInfo := getCallLogScheme(clusterHandler.RegionInfo.Region, call.CLUSTER, clusterIID.NameId, "ListNodeGroup()")

// 	start := call.Start()
// 	node_group_info_list := []*irs.NodeGroupInfo{}
// 	res, err := tencent.ListNodeGroup(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId)
// 	callLogInfo.ElapsedTime = call.Elapsed(start)
// 	if err != nil {
// 		err := fmt.Errorf("Failed to List Node Group :  %v", err)
// 		cblogger.Error(err)
// 		callLogInfo.ErrorMSG = err.Error()
// 		calllogger.Error(call.String(callLogInfo))
// 		return node_group_info_list, err
// 	}
// 	calllogger.Info(call.String(callLogInfo))

// 	for _, node_group := range res.Response.NodePoolSet {
// 		node_group_info, err := getNodeGroupInfo(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId, *node_group.NodePoolId)
// 		if err != nil {
// 			err := fmt.Errorf("Failed to Get Node Group Info:  %v", err)
// 			cblogger.Error(err)
// 			return nil, err
// 		}
// 		node_group_info_list = append(node_group_info_list, node_group_info)
// 	}

// 	return node_group_info_list, nil
// }

// func (clusterHandler *TencentClusterHandler) GetNodeGroup(clusterIID irs.IID, nodeGroupIID irs.IID) (irs.NodeGroupInfo, error) {
// 	cblogger.Info("Tencent Cloud Driver: called GetNodeGroup()")
// 	callLogInfo := getCallLogScheme(clusterHandler.RegionInfo.Region, call.CLUSTER, clusterIID.NameId, "GetNodeGroup()")

// 	start := call.Start()
// 	temp, err := getNodeGroupInfo(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId, nodeGroupIID.SystemId)
// 	callLogInfo.ElapsedTime = call.Elapsed(start)
// 	if err != nil {
// 		err := fmt.Errorf("Failed to Get Node Group Info:  %v", err)
// 		cblogger.Error(err)
// 		callLogInfo.ErrorMSG = err.Error()
// 		calllogger.Error(call.String(callLogInfo))
// 		return irs.NodeGroupInfo{}, err
// 	}
// 	calllogger.Info(call.String(callLogInfo))

// 	return *temp, nil
// }

func (clusterHandler *TencentClusterHandler) SetNodeGroupAutoScaling(clusterIID irs.IID, nodeGroupIID irs.IID, on bool) (bool, error) {
	cblogger.Info("Tencent Cloud Driver: called SetNodeGroupAutoScaling()")
	callLogInfo := GetCallLogScheme(clusterHandler.RegionInfo, call.CLUSTER, clusterIID.NameId, "SetNodeGroupAutoScaling()")

	start := call.Start()
	temp, err := tencent.SetNodeGroupAutoScaling(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId, nodeGroupIID.SystemId, on)
	callLogInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		err := fmt.Errorf("Failed to Set Node Group AutoScaling:  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return false, err
	}
	cblogger.Debug(temp.ToJsonString())
	calllogger.Info(call.String(callLogInfo))

	return true, nil
}

func (clusterHandler *TencentClusterHandler) ChangeNodeGroupScaling(clusterIID irs.IID, nodeGroupIID irs.IID, desiredNodeSize int, minNodeSize int, maxNodeSize int) (irs.NodeGroupInfo, error) {
	cblogger.Info("Tencent Cloud Driver: called ChangeNodeGroupScaling()")
	callLogInfo := GetCallLogScheme(clusterHandler.RegionInfo, call.CLUSTER, clusterIID.NameId, "ChangeNodeGroupScaling()")

	start := call.Start()

	//
	// Validation
	//
	err := validateAtChangeNodeGroupScaling(clusterIID, nodeGroupIID, minNodeSize, maxNodeSize)
	if err != nil {
		err := fmt.Errorf("Failed to Change Node Group Scaling:  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return irs.NodeGroupInfo{}, err
	}

	nodegroup, err := tencent.GetNodeGroup(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId, nodeGroupIID.SystemId)
	if err != nil {
		err := fmt.Errorf("Failed to Get Node Group:  %v", err)
		cblogger.Error(err)
		return irs.NodeGroupInfo{}, err
	}
	temp, err := tencent.ChangeNodeGroupScaling(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, *nodegroup.Response.NodePool.AutoscalingGroupId, uint64(desiredNodeSize), uint64(minNodeSize), uint64(maxNodeSize))
	callLogInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		err := fmt.Errorf("Failed to Change Node Group Scaling:  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return irs.NodeGroupInfo{}, err
	}
	cblogger.Debug(temp.ToJsonString())
	calllogger.Info(call.String(callLogInfo))

	node_group_info, err := getNodeGroupInfo(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId, nodeGroupIID.SystemId)
	if err != nil {
		err := fmt.Errorf("Failed to Get NodeGroupInfo:  %v", err)
		cblogger.Error(err)
		return irs.NodeGroupInfo{}, err
	}

	return *node_group_info, nil
}

func (clusterHandler *TencentClusterHandler) RemoveNodeGroup(clusterIID irs.IID, nodeGroupIID irs.IID) (bool, error) {
	cblogger.Info("Tencent Cloud Driver: called RemoveNodeGroup()")
	callLogInfo := GetCallLogScheme(clusterHandler.RegionInfo, call.CLUSTER, clusterIID.NameId, "RemoveNodeGroup()")

	start := call.Start()
	res, err := tencent.DeleteNodeGroup(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId, nodeGroupIID.SystemId)
	callLogInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		err := fmt.Errorf("Failed to Delete NodeGroup:  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return false, err
	}
	cblogger.Debug(res.ToJsonString())
	calllogger.Info(call.String(callLogInfo))

	return true, nil
}

func (clusterHandler *TencentClusterHandler) UpgradeCluster(clusterIID irs.IID, newVersion string) (irs.ClusterInfo, error) {
	cblogger.Info("Tencent Cloud Driver: called UpgradeCluster()")
	callLogInfo := GetCallLogScheme(clusterHandler.RegionInfo, call.CLUSTER, clusterIID.NameId, "UpgradeCluster()")

	start := call.Start()
	res, err := tencent.UpgradeCluster(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId, newVersion)
	callLogInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		err := fmt.Errorf("Failed to Upgrade Cluster:  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return irs.ClusterInfo{}, err
	}
	cblogger.Debug(res.ToJsonString())
	calllogger.Info(call.String(callLogInfo))

	clusterInfo, err := getClusterInfo(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region, clusterIID.SystemId)
	if err != nil {
		err := fmt.Errorf("Failed to Get ClusterInfo:  %v", err)
		cblogger.Error(err)
		return irs.ClusterInfo{}, err
	}

	return *clusterInfo, nil
}

func getClusterInfo(access_key string, access_secret string, region_id string, cluster_id string) (clusterInfo *irs.ClusterInfo, err error) {

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Failed to Process getClusterInfo() : %v\n\n%v", r, string(debug.Stack()))
			cblogger.Error(err)
		}
	}()

	res, err := tencent.GetCluster(access_key, access_secret, region_id, cluster_id)
	if err != nil {
		err := fmt.Errorf("Failed to Get Cluster:  %v", err)
		cblogger.Error(err)
		return nil, err
	}

	if *res.Response.TotalCount == 0 {
		err := fmt.Errorf("Failed to Get Cluster: cluster_id: %s", cluster_id)
		cblogger.Error(err)
		return nil, err
	}

	// https://intl.cloud.tencent.com/document/api/457/32022#ClusterStatus
	// Cluster status (Running, Creating, Idling or Abnormal)
	health_status := *res.Response.Clusters[0].ClusterStatus
	cluster_status := irs.ClusterActive
	if strings.EqualFold(health_status, "Creating") {
		cluster_status = irs.ClusterCreating
	} else if strings.EqualFold(health_status, "Upgrading") {
		cluster_status = irs.ClusterUpdating
	} else if strings.EqualFold(health_status, "Deleting") {
		cluster_status = irs.ClusterDeleting
	} else if strings.EqualFold(health_status, "Running") {
		cluster_status = irs.ClusterActive
	} else {
		cluster_status = irs.ClusterInactive
	}
	// else if strings.EqualFold(health_status, "") { // tencent has no "delete" state
	// cluster_status = irs.ClusterDeleting
	//}

	created_at := *res.Response.Clusters[0].CreatedTime // "2022-09-09T13:10:06Z",
	datetime, err := time.Parse(time.RFC3339, created_at)
	if err != nil {
		err := fmt.Errorf("Failed to Parse Create Time :  %v", err)
		cblogger.Error(err)
		panic(err)
	}
	jsonRes, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		cblogger.Error(fmt.Sprintf("Failed to marshal res: %v", err))
	} else {
		cblogger.Info(fmt.Sprintf("res in JSON format: %s", string(jsonRes)))
	}
	// description에서 security group 이름 추출
	security_group_id := ""
	re := regexp.MustCompile(`\S*#CB-SPIDER:PMKS:SECURITYGROUP:ID:\S*`)
	found := re.FindString(*res.Response.Clusters[0].ClusterDescription)
	if found != "" {
		split := strings.Split(found, "#CB-SPIDER:PMKS:SECURITYGROUP:ID:")
		security_group_id = split[1]
	}

	subnet_id := ""
	re = regexp.MustCompile(`\S*#CB-SPIDER:PMKS:SUBNET:ID:\S*`)
	found = re.FindString(*res.Response.Clusters[0].ClusterDescription)
	if found != "" {
		split := strings.Split(found, "#CB-SPIDER:PMKS:SUBNET:ID:")
		subnet_id = split[1]
	}

	accessInfo, err := getClusterAccessInfo(access_key, access_secret, region_id, cluster_id, security_group_id)
	if err != nil {
		cblogger.Error(err)
		return nil, err
	}

	clusterInfo = &irs.ClusterInfo{
		IId: irs.IID{
			NameId:   *res.Response.Clusters[0].ClusterName,
			SystemId: *res.Response.Clusters[0].ClusterId,
		},
		Version: *res.Response.Clusters[0].ClusterVersion,
		Network: irs.NetworkInfo{
			VpcIID: irs.IID{
				NameId:   "",
				SystemId: *res.Response.Clusters[0].ClusterNetworkSettings.VpcId,
			},
			SecurityGroupIIDs: []irs.IID{{NameId: "", SystemId: security_group_id}},
			SubnetIIDs:        []irs.IID{{NameId: "", SystemId: subnet_id}},
		},
		Status:      cluster_status,
		CreatedTime: datetime,
		AccessInfo:  accessInfo,
		// KeyValueList: []irs.KeyValue{}, // flatten data 입력하기
	}

	// Tags가 있는지 확인하고 추가
	if res.Response.Clusters[0].TagSpecification != nil {
		var tagList []irs.KeyValue
		for _, tag := range res.Response.Clusters[0].TagSpecification {

			for _, tag := range tag.Tags {
				tagList = append(clusterInfo.KeyValueList, irs.KeyValue{
					Key:   *tag.Key,
					Value: *tag.Value,
				})
			}
			clusterInfo.TagList = tagList
		}
	}

	// 2025-03-13 StructToKeyValueList 사용으로 변경
	clusterInfo.KeyValueList = irs.StructToKeyValueList(res.Response.Clusters[0])
	// k,v 추출 & 추가

	// KeyValueList: []irs.KeyValue{}, // flatten data 입력하기
	// temp, err := json.Marshal(*res.Response.Clusters[0])
	// if err != nil {
	// 	err := fmt.Errorf("Failed to Marshal Cluster Info :  %v", err)
	// 	cblogger.Error(err)
	// 	panic(err)
	// }
	// var json_obj map[string]interface{}
	// json.Unmarshal([]byte(temp), &json_obj)

	// flat, err := flatten.Flatten(json_obj, "", flatten.DotStyle)
	// if err != nil {
	// 	err := fmt.Errorf("Failed to Flatten Cluster Info :  %v", err)
	// 	cblogger.Error(err)
	// 	return nil, err
	// }
	// for k, v := range flat {
	// 	temp := fmt.Sprintf("%v", v)
	// 	clusterInfo.KeyValueList = append(clusterInfo.KeyValueList, irs.KeyValue{Key: k, Value: temp})
	// }

	clusterInfo.KeyValueList = irs.StructToKeyValueList(*res.Response.Clusters[0])


	// NodeGroups
	res2, err := tencent.ListNodeGroup(access_key, access_secret, region_id, cluster_id)
	if err != nil {
		err := fmt.Errorf("Failed to List Node Group :  %v", err)
		cblogger.Error(err)
		return nil, err
	}

	for _, nodepool := range res2.Response.NodePoolSet {
		node_group_info, err := getNodeGroupInfo(access_key, access_secret, region_id, cluster_id, *nodepool.NodePoolId)
		if err != nil {
			err := fmt.Errorf("Failed to Get Node Group Info :  %v", err)
			cblogger.Error(err)
			return nil, err
		}
		clusterInfo.NodeGroupList = append(clusterInfo.NodeGroupList, *node_group_info)
	}

	return clusterInfo, err
}

func getClusterAccessInfo(access_key string, access_secret string, region_id string, cluster_id string, security_group_id string) (accessInfo irs.AccessInfo, err error) {

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Failed to Process getClusterAccessInfo() : %v\n\n%v", r, string(debug.Stack()))
			cblogger.Error(err)
		}
	}()

	accessInfo = irs.AccessInfo{
		Endpoint:   "Endpoint is not ready yet!",
		Kubeconfig: "Kubeconfig is not ready yet!",
	}

	// (1) Endpoint
	res, err := tencent.GetClusterEndpoint(access_key, access_secret, region_id, cluster_id)
	if err != nil {
		if strings.Contains(err.Error(), "CLUSTER_IN_ABNORMAL_STAT") || strings.Contains(err.Error(), "CLUSTER_STATE_ERROR") {
			cblogger.Info(cluster_id + err.Error())
			accessInfo.Endpoint = "Cluster is not ready yet!"
		} else {
			err := fmt.Errorf("Failed to Get Cluster Endpoint:  %v", err)
			cblogger.Error(err)
			return irs.AccessInfo{}, err
		}
	}

	if res == nil || res.Response == nil {
		return accessInfo, nil
	}

	if *res.Response.ClusterExternalEndpoint == "" {
		_, err := tencent.CreateClusterEndpoint(access_key, access_secret, region_id, cluster_id, security_group_id)
		if err != nil {
			if strings.Contains(err.Error(), "CLUSTER_IN_ABNORMAL_STAT") || strings.Contains(err.Error(), "CLUSTER_STATE_ERROR") {
				cblogger.Info(cluster_id + err.Error())
				accessInfo.Endpoint = "First, add a nodegroup."
			} else if strings.Contains(err.Error(), "same type task in execution") {
				cblogger.Error(cluster_id + err.Error())
				accessInfo.Endpoint = "Preparing...."
			} else {
				err := fmt.Errorf("Failed to Create Cluster Endpoint:  %v", err)
				cblogger.Error(err)
				return irs.AccessInfo{}, err
			}
		}
	} else {
		accessInfo.Endpoint = *res.Response.ClusterExternalEndpoint
	}

	// (2) Kubeconfig
	resKubeconfig, err := tencent.GetClusterKubeconfig(access_key, access_secret, region_id, cluster_id)
	if err != nil {
		if strings.Contains(err.Error(), "CLUSTER_IN_ABNORMAL_STAT") || strings.Contains(err.Error(), "CLUSTER_STATE_ERROR") {
			cblogger.Info(cluster_id + err.Error())
			accessInfo.Kubeconfig = "Cluster is not ready yet!"
		} else {
			err := fmt.Errorf("Failed to Get Cluster Kubeconfig:  %v", err)
			cblogger.Error(err)
			return irs.AccessInfo{}, err
		}
	}

	if resKubeconfig == "" {
		return accessInfo, nil
	}

	if resKubeconfig == "" {
		accessInfo.Kubeconfig = "Preparing...."
	} else {
		accessInfo.Kubeconfig = changeDomainNameToIP(resKubeconfig, accessInfo.Endpoint)
	}

	return accessInfo, nil
}

func changeDomainNameToIP(kubeConfig string, endpoint string) string {

	TargetStr := "    server: https://"

	if kubeConfig == "" || !strings.Contains(kubeConfig, TargetStr) {
		return kubeConfig
	}
	if endpoint == "" || !strings.Contains(endpoint, ":") {
		return kubeConfig
	}

	// get IP from 1.2.3.4:443
	splits := strings.Split(endpoint, ":")
	ip := splits[0]

	// replace 'domain name' with 'ip'
	// ex) server: https://cls-amu0j0tf.ccs.tencent-cloud.com
	//     => server: https://1.2.3.4
	lines := strings.Split(kubeConfig, "\n")
	for i, line := range lines {
		if strings.Contains(line, TargetStr) {
			lines[i] = TargetStr + ip
		}
	}

	return strings.Join(lines, "\n")
}

func getNodeGroupInfo(access_key, access_secret, region_id, cluster_id, node_group_id string) (nodeGroupInfo *irs.NodeGroupInfo, err error) {

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Failed to Process getNodeGroupInfo() : %v\n\n%v", r, string(debug.Stack()))
			cblogger.Error(err)
		}
	}()

	res, err := tencent.GetNodeGroup(access_key, access_secret, region_id, cluster_id, node_group_id)
	if err != nil {
		err := fmt.Errorf("Failed to Get Node Group :  %v", err)
		cblogger.Error(err)
		return nil, err
	}

	launch_config, err := tencent.GetLaunchConfiguration(access_key, access_secret, region_id, *res.Response.NodePool.LaunchConfigurationId)
	if err != nil {
		err := fmt.Errorf("Failed to Get Launch Configuration :  %v", err)
		cblogger.Error(err)
		return nil, err
	}

	auto_scaling_group, err := tencent.GetAutoScalingGroup(access_key, access_secret, region_id, *res.Response.NodePool.AutoscalingGroupId)
	if err != nil {
		err := fmt.Errorf("Failed to Get Auto Scaling Group :  %v", err)
		cblogger.Error(err)
		return nil, err
	}

	// nodepool LifeState
	// The lifecycle state of the current node pool.
	// Valid values: creating, normal, updating, deleting, and deleted.
	health_status := *res.Response.NodePool.LifeState
	status := irs.NodeGroupActive
	if strings.EqualFold(health_status, "normal") {
		status = irs.NodeGroupActive
	} else if strings.EqualFold(health_status, "creating") {
		status = irs.NodeGroupUpdating
	} else if strings.EqualFold(health_status, "removing") {
		status = irs.NodeGroupUpdating // removing is a kind of updating?
	} else if strings.EqualFold(health_status, "deleting") {
		status = irs.NodeGroupDeleting
	} else if strings.EqualFold(health_status, "updating") {
		status = irs.NodeGroupUpdating
	}

	auto_scale_enalbed := false
	if strings.EqualFold(*res.Response.NodePool.AutoscalingGroupStatus, "ENABLED") {
		auto_scale_enalbed = true
	}

	if len(launch_config.Response.LaunchConfigurationSet) > 0 && len(auto_scaling_group.Response.AutoScalingGroupSet) > 0 {
		nodeGroupInfo = &irs.NodeGroupInfo{
			IId: irs.IID{
				NameId:   *res.Response.NodePool.Name,
				SystemId: *res.Response.NodePool.NodePoolId,
			},
			ImageIID: irs.IID{
				NameId:   "",
				SystemId: *launch_config.Response.LaunchConfigurationSet[0].ImageId,
			},
			VMSpecName:      *launch_config.Response.LaunchConfigurationSet[0].InstanceType,
			RootDiskType:    *launch_config.Response.LaunchConfigurationSet[0].SystemDisk.DiskType,
			RootDiskSize:    fmt.Sprintf("%d", *launch_config.Response.LaunchConfigurationSet[0].SystemDisk.DiskSize),
			KeyPairIID:      irs.IID{NameId: "", SystemId: *launch_config.Response.LaunchConfigurationSet[0].LoginSettings.KeyIds[0]},
			Status:          status,
			OnAutoScaling:   auto_scale_enalbed,
			MinNodeSize:     int(*auto_scaling_group.Response.AutoScalingGroupSet[0].MinSize),
			MaxNodeSize:     int(*auto_scaling_group.Response.AutoScalingGroupSet[0].MaxSize),
			DesiredNodeSize: int(*auto_scaling_group.Response.AutoScalingGroupSet[0].DesiredCapacity),
			Nodes:           []irs.IID{}, // to be implemented
			KeyValueList:    []irs.KeyValue{},
		}
	}

	nodes, err := tencent.DescribeClusterInstances(access_key, access_secret, region_id, cluster_id)
	if err != nil {
		err := fmt.Errorf("Failed to Get Nodes :  %v", err)
		cblogger.Error(err)
		return nil, err
	}
	for _, node := range nodes.Response.InstanceSet {
		if node_group_id == *node.NodePoolId {
			if *node.InstanceId != "" {
				nodeGroupInfo.Nodes = append(nodeGroupInfo.Nodes, irs.IID{NameId: "", SystemId: *node.InstanceId})
			}
		}
	}

	// 2025-03-13 StructToKeyValueList 사용으로 변경
	nodeGroupInfo.KeyValueList = irs.StructToKeyValueList(res.Response.NodePool)
	// add key value list

	// temp, err := json.Marshal(*res.Response.NodePool)
	// if err != nil {
	// 	err := fmt.Errorf("Failed to Marshal NodeGroup Info :  %v", err)
	// 	cblogger.Error(err)
	// 	panic(err)
	// }
	// var json_obj map[string]interface{}
	// json.Unmarshal([]byte(temp), &json_obj)
	// flat, err := flatten.Flatten(json_obj, "", flatten.DotStyle)
	// if err != nil {
	// 	err := fmt.Errorf("Failed to Flatten NodeGroup Info :  %v", err)
	// 	cblogger.Error(err)
	// 	return nil, err
	// }
	// for k, v := range flat {
	// 	temp := fmt.Sprintf("%v", v)
	// 	nodeGroupInfo.KeyValueList = append(nodeGroupInfo.KeyValueList, irs.KeyValue{Key: k, Value: temp})
	// }

	nodeGroupInfo.KeyValueList = irs.StructToKeyValueList(*res.Response.NodePool)


	return nodeGroupInfo, err
}

func getCreateClusterRequest(clusterHandler *TencentClusterHandler, clusterInfo irs.ClusterInfo) (request *tke.CreateClusterRequest, err error) {

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Failed to Process getCreateClusterRequest() : %v\n\n%v", r, string(debug.Stack()))
			cblogger.Error(err)
		}
	}()

	// 172.X.0.0.16: X Range:16, 17, ... , 31
	m_cidr := make(map[string]bool)
	for i := 16; i < 32; i++ {
		m_cidr[fmt.Sprintf("172.%v.0.0/16", i)] = true
	}

	clusters, err := clusterHandler.ListCluster()
	if err != nil {
		err := fmt.Errorf("Failed to List Cluster :  %v", err)
		cblogger.Error(err)
		return nil, err
	}
	for _, cluster := range clusters {
		for _, v := range cluster.KeyValueList {
			if v.Key == "ClusterNetworkSettings.ClusterCIDR" {
				delete(m_cidr, v.Value)
			}
		}
	}

	cidr_list := []string{}
	for k := range m_cidr {
		cidr_list = append(cidr_list, k)
	}

	request = tke.NewCreateClusterRequest()
	request.ClusterCIDRSettings = &tke.ClusterCIDRSettings{
		ClusterCIDR: common.StringPtr(cidr_list[0]), // 172.X.0.0.16: X Range:16, 17, ... , 31
	}

	// security_group_name을 저장하는 방법이 없음.
	// description에 securityp_group_name을 저장해서 사용함.
	// 향후, 추가 정보가 필요하면, description에 json 문서를 저장하는 방식으로 사용할 수도 있음.
	//
	// 정보검색은
	// 사용자가 필요에 따라서 다른 description내용을 추가할 수 도 있으니,
	// "#CB-SPIDER:PMKS:SECURITYGROUP:ID"을 포함하는 Line을 찾아서 처리
	// >> regex로 구현
	// ------------------------------------------------------------
	// subnet_id 저장이 안됨
	// description 정보에 저장해서 사용
	// SubnetId:       common.StringPtr(clusterInfo.Network.SubnetIIDs[0].SystemId),
	// " #CB-SPIDER:PMKS:SUBNET:ID:"
	desc_str := `#CB-SPIDER:PMKS:SECURITYGROUP:ID:%s #CB-SPIDER:PMKS:SUBNET:ID:%s`
	desc_str = fmt.Sprintf(desc_str, clusterInfo.Network.SecurityGroupIIDs[0].SystemId, clusterInfo.Network.SubnetIIDs[0].SystemId)

	var tags []*tke.Tag
	for _, inputTag := range clusterInfo.TagList {
		tags = append(tags, &tke.Tag{
			Key:   common.StringPtr(inputTag.Key),
			Value: common.StringPtr(inputTag.Value),
		})
	}

	request.ClusterBasicSettings = &tke.ClusterBasicSettings{
		ClusterName:        common.StringPtr(clusterInfo.IId.NameId),
		VpcId:              common.StringPtr(clusterInfo.Network.VpcIID.SystemId),
		ClusterVersion:     common.StringPtr(clusterInfo.Version), // option, version: 1.22.5
		ClusterDescription: common.StringPtr(desc_str),            // option, #CB-SPIDER:PMKS:SECURITYGROUP:sg-c00t00ih
		TagSpecification: []*tke.TagSpecification{{
			ResourceType: common.StringPtr("cluster"),
			Tags:         tags,
		}},
	}
	request.ClusterType = common.StringPtr("MANAGED_CLUSTER") //default value
	request.ClusterAdvancedSettings = &tke.ClusterAdvancedSettings{
		ContainerRuntime: common.StringPtr(defaultContainerRuntime),
	}

	return request, err
}

func getNodeGroupRequest(clusterHandler *TencentClusterHandler, cluster_id string, nodeGroupReqInfo irs.NodeGroupInfo) (request *tke.CreateClusterNodePoolRequest, err error) {

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Failed to Process getNodeGroupRequest() : %v\n\n%v", r, string(debug.Stack()))
			cblogger.Error(err)
		}
	}()

	cluster, res := clusterHandler.GetCluster(irs.IID{SystemId: cluster_id})
	if res != nil {
		err := fmt.Errorf("Failed to Get Cluster :  %v", err)
		cblogger.Error(err)
		return nil, res
	}
	vpc_id := cluster.Network.VpcIID.SystemId
	subnet_id := cluster.Network.SubnetIIDs[0].SystemId
	security_group_id := cluster.Network.SecurityGroupIIDs[0].SystemId
	disk_size, _ := strconv.ParseInt(nodeGroupReqInfo.RootDiskSize, 10, 64)

	strSystemDisk := ""
	switch {
	case nodeGroupReqInfo.RootDiskType == "" && disk_size == 0:
		strSystemDisk = ""
	case nodeGroupReqInfo.RootDiskType == "" && disk_size != 0:
		strSystemDisk = `"SystemDisk": { "DiskSize": %d },`
		strSystemDisk = fmt.Sprintf(strSystemDisk, disk_size)
	case nodeGroupReqInfo.RootDiskType != "" && disk_size == 0:
		strSystemDisk = `"SystemDisk": { "DiskType" : "%s" },`
		strSystemDisk = fmt.Sprintf(strSystemDisk, nodeGroupReqInfo.RootDiskType)
	default:
		strSystemDisk = `"SystemDisk": { "DiskType" : "%s", "DiskSize": %d },`
		strSystemDisk = fmt.Sprintf(strSystemDisk, nodeGroupReqInfo.RootDiskType, disk_size)
	}

	// '{"LaunchConfigurationName":"name","InstanceType":"S3.MEDIUM2","ImageId":"img-pi0ii46r"}'
	// "SystemDisk": { "DiskType" : "CLOUD_BSSD", "DiskSize": 50 },
	// ImageId를 설정하면 에러 발생, 설정안됨.
	launch_config_json_str := `{
		"InstanceType": "%s",
		"SecurityGroupIds": ["%s"],
		"LoginSettings": { "KeyIds" : ["%s"] },
		"InstanceChargeType": "POSTPAID_BY_HOUR",
		%s
		"InternetAccessible": {
			"InternetChargeType":"TRAFFIC_POSTPAID_BY_HOUR",
			"InternetMaxBandwidthOut": 1,
			"PublicIpAssigned": true
		}
	}`
	launch_config_json_str = fmt.Sprintf(launch_config_json_str, nodeGroupReqInfo.VMSpecName, security_group_id, nodeGroupReqInfo.KeyPairIID.SystemId, strSystemDisk)

	auto_scaling_group_json_str := `{
		"MinSize": %d,
		"MaxSize": %d,			
		"DesiredCapacity": %d,
		"VpcId": "%s",
		"SubnetIds": ["%s"]
	}`

	auto_scaling_group_json_str = fmt.Sprintf(auto_scaling_group_json_str, nodeGroupReqInfo.MinNodeSize, nodeGroupReqInfo.MaxNodeSize, nodeGroupReqInfo.DesiredNodeSize, vpc_id, subnet_id)

	request = tke.NewCreateClusterNodePoolRequest()
	request.Name = common.StringPtr(nodeGroupReqInfo.IId.NameId)
	request.ClusterId = common.StringPtr(cluster_id)
	request.LaunchConfigurePara = common.StringPtr(launch_config_json_str)
	request.AutoScalingGroupPara = common.StringPtr(auto_scaling_group_json_str)
	request.EnableAutoscale = common.BoolPtr(nodeGroupReqInfo.OnAutoScaling)
	request.InstanceAdvancedSettings = &tke.InstanceAdvancedSettings{
		// DataDisks: []*tke.DataDisk{
		// 	{
		// 		DiskType: common.StringPtr(nodeGroupReqInfo.RootDiskType), //ex. "CLOUD_PREMIUM"
		// 		DiskSize: common.Int64Ptr(disk_size),                      //ex. 50
		// 	},
		// },
	}
	if nodeGroupReqInfo.ImageIID.SystemId != "" {
		// 등록 가능한 이미지 이름 목록: https://www.tencentcloud.com/document/product/457/46750
		request.NodePoolOs = common.StringPtr(nodeGroupReqInfo.ImageIID.SystemId) // ex: "tlinux3.1x86_64"
	}
	// request.ContainerRuntime = common.StringPtr("docker")
	// request.RuntimeVersion = common.StringPtr("19.3")
	// print(request.ToJsonString())

	return request, err
}

// func getCallLogScheme(region string, resourceType call.RES_TYPE, resourceName string, apiName string) call.CLOUDLOGSCHEMA {
// 	cblogger.Info(fmt.Sprintf("Call %s %s", call.TENCENT, apiName))
// 	return call.CLOUDLOGSCHEMA{
// 		CloudOS:      call.TENCENT,
// 		RegionZone:   region,
// 		ResourceType: resourceType,
// 		ResourceName: resourceName,
// 		CloudOSAPI:   apiName,
// 	}
// }

func validateAtCreateCluster(clusterInfo irs.ClusterInfo) error {
	if clusterInfo.IId.NameId == "" {
		return fmt.Errorf("Cluster name is required")
	}
	if clusterInfo.Network.VpcIID.SystemId == "" && clusterInfo.Network.VpcIID.NameId == "" {
		return fmt.Errorf("Cannot identify VPC(IID=%s)", clusterInfo.Network.VpcIID)
	}
	if len(clusterInfo.Network.SubnetIIDs) < 1 {
		return fmt.Errorf("At least one Subnet must be specified")
	}
	if len(clusterInfo.Network.SecurityGroupIIDs) < 1 {
		return fmt.Errorf("At least one Subnet must be specified")
	}
	// CAUTION: Currently CB-Spider's Tencent PMKS Drivers does not support to create a cluster with nodegroups
	if len(clusterInfo.NodeGroupList) > 0 {
		return fmt.Errorf("Node Group cannot be specified")
	}

	return nil
}

func validateAtAddNodeGroup(clusterIID irs.IID, nodeGroupInfo irs.NodeGroupInfo) error {
	if clusterIID.SystemId == "" && clusterIID.NameId == "" {
		return fmt.Errorf("Invalid Cluster IID")
	}
	if nodeGroupInfo.IId.NameId == "" {
		return fmt.Errorf("Node Group name is required")
	}
	if nodeGroupInfo.MaxNodeSize < 1 {
		return fmt.Errorf("MaxNodeSize cannot be smaller than 1")
	}
	if nodeGroupInfo.MinNodeSize < 1 {
		return fmt.Errorf("MaxNodeSize cannot be smaller than 1")
	}
	if nodeGroupInfo.DesiredNodeSize < 1 {
		return fmt.Errorf("DesiredNodeSize cannot be smaller than 1")
	}
	if nodeGroupInfo.VMSpecName == "" {
		return fmt.Errorf("VM Spec Name is required")
	}

	return nil
}

func validateAtChangeNodeGroupScaling(clusterIID irs.IID, nodeGroupIID irs.IID, minNodeSize int, maxNodeSize int) error {
	if clusterIID.SystemId == "" && clusterIID.NameId == "" {
		return fmt.Errorf("Invalid Cluster IID")
	}
	if nodeGroupIID.SystemId == "" && nodeGroupIID.NameId == "" {
		return fmt.Errorf("Invalid Node Group IID")
	}
	if minNodeSize < 1 {
		return fmt.Errorf("MaxNodeSize cannot be smaller than 1")
	}
	if maxNodeSize < 1 {
		return fmt.Errorf("MaxNodeSize cannot be smaller than 1")
	}

	return nil
}

func (clusterHandler *TencentClusterHandler) ListIID() ([]*irs.IID, error) {
	var iidList []*irs.IID

	callLogInfo := GetCallLogScheme(clusterHandler.RegionInfo, call.CLUSTER, "ListIID", "GetClusters()")

	start := call.Start()
	res, err := tencent.GetClusters(clusterHandler.CredentialInfo.ClientId, clusterHandler.CredentialInfo.ClientSecret, clusterHandler.RegionInfo.Region)
	callLogInfo.ElapsedTime = call.Elapsed(start)
	if err != nil {
		err := fmt.Errorf("Failed to Get Clusters :  %v", err)
		cblogger.Error(err)
		callLogInfo.ErrorMSG = err.Error()
		calllogger.Error(call.String(callLogInfo))
		return iidList, err
	}
	calllogger.Debug(call.String(callLogInfo))

	for _, cluster := range res.Response.Clusters {
		iid := irs.IID{SystemId: *cluster.ClusterId}
		iidList = append(iidList, &iid)
	}
	return iidList, nil
}
