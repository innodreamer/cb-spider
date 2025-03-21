// Mock Driver of CB-Spider.
// The CB-Spider is a sub-Framework of the Cloud-Barista Multi-Cloud Project.
// The CB-Spider Mission is to connect all the clouds with a single interface.
//
//      * Cloud-Barista: https://github.com/cloud-barista
//
// This is Mock Driver.
//
// by CB-Spider Team, 2020.09.

package mock

import (
	cblog "github.com/cloud-barista/cb-log"
	"github.com/sirupsen/logrus"

	mkcon "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/drivers/mock/connect"
	mkrs "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/drivers/mock/resources"
	idrv "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/interfaces"
	icon "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/interfaces/connect"
	ires "github.com/cloud-barista/cb-spider/cloud-control-manager/cloud-driver/interfaces/resources"
)

type MockDriver struct{}

var cblogger *logrus.Logger

func init() {
	// cblog is a global variable.
	cblogger = cblog.GetLogger("CB-SPIDER")
}

func (MockDriver) GetDriverVersion() string {
	return "MOCK DRIVER Version 1.0"
}

func (MockDriver) GetDriverCapability() idrv.DriverCapabilityInfo {
	var drvCapabilityInfo idrv.DriverCapabilityInfo

	drvCapabilityInfo.ZoneBasedControl = true

	drvCapabilityInfo.RegionZoneHandler = true
	drvCapabilityInfo.PriceInfoHandler = true
	drvCapabilityInfo.ImageHandler = true
	drvCapabilityInfo.VMSpecHandler = true

	drvCapabilityInfo.VPCHandler = true
	drvCapabilityInfo.SecurityHandler = true
	drvCapabilityInfo.KeyPairHandler = true
	drvCapabilityInfo.VMHandler = true
	drvCapabilityInfo.DiskHandler = true
	drvCapabilityInfo.MyImageHandler = true
	drvCapabilityInfo.NLBHandler = true
	drvCapabilityInfo.ClusterHandler = true

	drvCapabilityInfo.TagHandler = true
	drvCapabilityInfo.TagSupportResourceType = []ires.RSType{ires.VPC, ires.SUBNET, ires.SG, ires.KEY, ires.VM, ires.NLB, ires.DISK, ires.MYIMAGE, ires.CLUSTER}

	drvCapabilityInfo.VPC_CIDR = true

	return drvCapabilityInfo
}

func (driver *MockDriver) ConnectCloud(connectionInfo idrv.ConnectionInfo) (icon.CloudConnection, error) {
	// <standard flow>
	// 1. get info of credential and region for Test A Cloud from connectionInfo.
	// 2. create a client object(or service  object) of XXX Cloud with credential info.
	// 3. create CloudConnection Instance of "connect/XXX_CloudConnection".
	// 4. return CloudConnection Interface of XXX_CloudConnection.

	// ex)
	// MockName = "mock01"
	iConn := mkcon.MockConnection{
		Region:   connectionInfo.RegionInfo,
		MockName: connectionInfo.CredentialInfo.MockName,
	}

	// Please, do not delete this line.
	mkrs.PrepareVMImage(iConn.MockName)
	mkrs.PrepareVMSpec(iConn.MockName)
	mkrs.PrepareRegionZone(iConn.MockName)

	return &iConn, nil
}
