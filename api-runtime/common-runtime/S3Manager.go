// Cloud Control Manager's Rest Runtime of CB-Spider.
// Common Runtime for S3 Management
// by CB-Spider Team

package commonruntime

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/cors"
	"github.com/minio/minio-go/v7/pkg/credentials"

	ccm "github.com/cloud-barista/cb-spider/cloud-control-manager"
	ccim "github.com/cloud-barista/cb-spider/cloud-info-manager/connection-config-info-manager"
	cim "github.com/cloud-barista/cb-spider/cloud-info-manager/credential-info-manager"
	rim "github.com/cloud-barista/cb-spider/cloud-info-manager/region-info-manager"
	infostore "github.com/cloud-barista/cb-spider/info-store"
)

type S3BucketIIDInfo struct {
	ConnectionName string `gorm:"primaryKey"`
	NameId         string `gorm:"primaryKey"`
	SystemId       string
	Region         string
	CreatedAt      time.Time
}

func (S3BucketIIDInfo) TableName() string {
	return "s3bucket_iid_infos"
}

func init() {
	db, err := infostore.Open()
	if err != nil {
		cblog.Error(err)
		return
	}
	db.AutoMigrate(&S3BucketIIDInfo{})
	infostore.Close(db)
}

type S3ConnectionInfo struct {
	Endpoint       string
	AccessKey      string
	SecretKey      string
	UseSSL         bool
	RegionRequired bool   // Indicates if region is required for the S3 service
	Region         string // Region ID
}

func GetS3ConnectionInfo(connectionName string) (*S3ConnectionInfo, error) {

	// Get connection configuration
	cccInfo, err := ccim.GetConnectionConfig(connectionName)
	if err != nil {
		return nil, err
	}

	// Get Region information
	regionInfo, err := rim.GetRegion(cccInfo.RegionName)
	if err != nil {
		return nil, err
	}
	regionID := ccm.KeyValueListGetValue(regionInfo.KeyValueInfoList, "Region")
	// Get decrypted credential information
	crdInfo, err := cim.GetCredentialDecrypt(cccInfo.CredentialName)
	if err != nil {
		return nil, err
	}

	endpoint := ccm.KeyValueListGetValue(crdInfo.KeyValueInfoList, "S3Endpoint")
	endpoint = strings.Replace(endpoint, "{region}", regionID, -1) // Replace {region} with actual region name
	endpoint = strings.Replace(endpoint, "{REGION}", regionID, -1) // Replace {REGION} with actual region name
	endpoint = strings.Replace(endpoint, "{Region}", regionID, -1) // Replace {Region} with actual region name

	return &S3ConnectionInfo{
		Endpoint:       endpoint,
		AccessKey:      ccm.KeyValueListGetValue(crdInfo.KeyValueInfoList, "S3AccessKey"),
		SecretKey:      ccm.KeyValueListGetValue(crdInfo.KeyValueInfoList, "S3SecretKey"),
		UseSSL:         ccm.KeyValueListGetValue(crdInfo.KeyValueInfoList, "S3UseSSL") == "true",
		RegionRequired: ccm.KeyValueListGetValue(crdInfo.KeyValueInfoList, "S3RegionRequired") == "true",
		Region:         regionID,
	}, nil

}

func NewS3Client(connInfo *S3ConnectionInfo) (*minio.Client, error) {
	if connInfo.RegionRequired {
		return minio.New(connInfo.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(connInfo.AccessKey, connInfo.SecretKey, ""),
			Secure: connInfo.UseSSL,
			Region: connInfo.Region,
		})
	} else { // Alibaba, IBM VPC, NCP VPC, KT VPC: Region is not required
		return minio.New(connInfo.Endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(connInfo.AccessKey, connInfo.SecretKey, ""),
			Secure: connInfo.UseSSL,
		})
	}
}

func CreateS3Bucket(connectionName, bucketName string) (*minio.BucketInfo, error) {
	cblog.Info("call CreateS3Bucket()")

	connectionName, err := EmptyCheckAndTrim("connectionName", connectionName)
	if err != nil {
		cblog.Error(err)
		return nil, err
	}
	exist, err := infostore.HasByConditions(&S3BucketIIDInfo{}, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		cblog.Error(err)
		return nil, err
	}
	if exist {
		return nil, fmt.Errorf("S3 Bucket %s already exists", bucketName)
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		cblog.Error(err)
		return nil, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		cblog.Error(err)
		return nil, err
	}
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		cblog.Error(err)
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("S3 Bucket %s already exists in S3", bucketName)
	}
	if connInfo.RegionRequired {
		if connInfo.Region == "" {
			return nil, fmt.Errorf("Region is required for S3 connection %s", connectionName)
		} else {
			err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: connInfo.Region})
		}
	} else { // Alibaba, IBM VPC, NCP VPC, KT VPC: Region is not required
		err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	}
	if err != nil {
		cblog.Error(err)
		return nil, err
	}
	iidInfo := S3BucketIIDInfo{
		ConnectionName: connectionName,
		NameId:         bucketName,
		SystemId:       bucketName,
		Region:         connInfo.Region,
		CreatedAt:      time.Now(),
	}
	err = infostore.Insert(&iidInfo)
	if err != nil {
		cblog.Error(err)
		_ = client.RemoveBucket(ctx, bucketName)
		return nil, err
	}
	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range buckets {
		if b.Name == bucketName {
			return &b, nil
		}
	}
	return nil, fmt.Errorf("Bucket %s created, but info not found", bucketName)
}

func ListS3Buckets(connectionName string) ([]*minio.BucketInfo, error) {
	cblog.Info("call ListS3Buckets()")

	var iidInfoList []*S3BucketIIDInfo
	err := infostore.ListByCondition(&iidInfoList, "connection_name", connectionName)
	if err != nil {
		return nil, err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return nil, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	allBuckets, err := client.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}

	var out []*minio.BucketInfo
	for _, iid := range iidInfoList {
		for _, b := range allBuckets {
			if b.Name == iid.NameId {
				out = append(out, &b)
				break
			}
		}
	}
	return out, nil
}

func GetS3Bucket(connectionName, bucketName string) (*minio.BucketInfo, error) {
	cblog.Info("call GetS3Bucket()")
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return nil, err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return nil, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range buckets {
		if b.Name == iidInfo.NameId {
			return &b, nil
		}
	}
	return nil, fmt.Errorf("Bucket %s not found", bucketName)
}

func DeleteS3Bucket(connectionName, bucketName string) (bool, error) {
	cblog.Info("call DeleteS3Bucket()")
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return false, err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return false, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return false, err
	}
	ctx := context.Background()
	err = client.RemoveBucket(ctx, bucketName)
	if err != nil {
		return false, err
	}
	_, err = infostore.DeleteByConditions(&S3BucketIIDInfo{}, "connection_name", iidInfo.ConnectionName, "name_id", bucketName)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ListS3Objects(connectionName, bucketName, prefix string) ([]minio.ObjectInfo, error) {
	cblog.Info("call ListS3Objects()")
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return nil, err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return nil, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}
	var out []minio.ObjectInfo
	for obj := range client.ListObjects(ctx, bucketName, opts) {
		if obj.Err != nil {
			continue
		}
		out = append(out, obj)
	}
	return out, nil
}

func GetS3ObjectInfo(connectionName, bucketName, objectName string) (*minio.ObjectInfo, error) {
	cblog.Info("call GetS3ObjectInfo()")
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return nil, err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return nil, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	stat, err := client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}
	return &stat, nil
}

func PutS3ObjectFromFile(connectionName, bucketName, objectName, filePath string) (*minio.UploadInfo, error) {
	cblog.Info("call PutS3ObjectFromFile()")
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return nil, err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return nil, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	info, err := client.FPutObject(ctx, bucketName, objectName, filePath, minio.PutObjectOptions{})
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func DeleteS3Object(connectionName, bucketName, objectName string) (bool, error) {
	cblog.Info("call DeleteS3Object()")
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return false, err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return false, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return false, err
	}
	ctx := context.Background()
	err = client.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetS3ObjectStream(connectionName, bucketName, objectName string) (io.ReadCloser, error) {
	cblog.Info("call GetS3ObjectStream()")
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return nil, err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return nil, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	obj, err := client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil // io.ReadCloser
}

func GetS3PresignedURL(connectionName, bucketName, objectName, method string, expiresSeconds int64, contentDisposition string) (string, error) {
	cblog.Info("call GetS3PresignedURL()")
	var iidInfo S3BucketIIDInfo
	if err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName); err != nil {
		return "", err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return "", err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	expires := time.Duration(expiresSeconds) * time.Second

	switch method {
	case "GET":
		var params url.Values
		if contentDisposition != "" {
			params = url.Values{}
			params.Set("response-content-disposition", contentDisposition)
		}
		u, err := client.PresignedGetObject(ctx, bucketName, objectName, expires, params)
		if err != nil {
			return "", err
		}
		return u.String(), nil

	case "PUT":
		u, err := client.PresignedPutObject(ctx, bucketName, objectName, expires)
		if err != nil {
			return "", err
		}
		return u.String(), nil

	default:
		return "", fmt.Errorf("Unsupported method: %s", method)
	}
}

func SetS3BucketACL(connectionName, bucketName, acl string) (string, error) {
	cblog.Info("call SetS3BucketACL()")
	// acl: "private", "public-read", etc.
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return "", err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return "", err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	// SetBucketPolicy for ACL: minio-go does not provide direct SetBucketACL, so map to policy
	var policy string
	switch acl {
	case "public-read":
		policy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":["s3:GetObject"],"Resource":["arn:aws:s3:::` + bucketName + `/*"]}]}`
	case "private":
		policy = `{"Version":"2012-10-17","Statement":[]}`
	default:
		return "", fmt.Errorf("unsupported ACL: %s", acl)
	}
	err = client.SetBucketPolicy(ctx, bucketName, policy)
	if err != nil {
		return "", err
	}
	appliedPolicy, err := client.GetBucketPolicy(ctx, bucketName)
	if err != nil {
		return "", err
	}
	return appliedPolicy, nil
}

func GetS3BucketACL(connectionName, bucketName string) (string, error) {
	cblog.Info("call GetS3BucketACL()")
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return "", err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return "", err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return "", err
	}
	ctx := context.Background()
	policy, err := client.GetBucketPolicy(ctx, bucketName)
	if err != nil {
		return "", err
	}
	return policy, nil
}

func EnableVersioning(connectionName, bucketName string) (bool, error) {
	cblog.Info("call EnableVersioning()")
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return false, err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return false, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return false, err
	}
	ctx := context.Background()
	opts := minio.BucketVersioningConfiguration{
		Status: "Enabled",
	}

	err = client.SetBucketVersioning(ctx, bucketName, opts)
	if err != nil {
		return false, err
	}
	return true, nil
}

func SuspendVersioning(connectionName, bucketName string) (bool, error) {
	cblog.Info("call SuspendVersioning()")
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return false, err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return false, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return false, err
	}
	ctx := context.Background()
	opts := minio.BucketVersioningConfiguration{
		Status: "Suspended",
	}
	err = client.SetBucketVersioning(ctx, bucketName, opts)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ListS3ObjectVersions(connectionName, bucketName, prefix string) ([]minio.ObjectInfo, error) {
	cblog.Info("call ListS3ObjectVersions()")
	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return nil, err
	}
	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return nil, err
	}
	client, err := NewS3Client(connInfo)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	opts := minio.ListObjectsOptions{
		Prefix:       prefix,
		Recursive:    true,
		WithVersions: true,
	}

	var out []minio.ObjectInfo
	for obj := range client.ListObjects(ctx, bucketName, opts) {
		if obj.Err != nil {
			continue
		}
		out = append(out, obj)
	}
	return out, nil
}

// PutS3ObjectFromReader uploads an object to S3 from io.Reader
func PutS3ObjectFromReader(connectionName string, bucketName string, objectName string, reader io.Reader, objectSize int64) (minio.UploadInfo, error) {
	cblog.Info("call PutS3ObjectFromReader()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return minio.UploadInfo{}, err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return minio.UploadInfo{}, err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return minio.UploadInfo{}, err
	}

	ctx := context.Background()
	contentType := "application/octet-stream"

	info, err := client.PutObject(
		ctx,
		bucketName,
		objectName,
		reader,
		objectSize,
		minio.PutObjectOptions{ContentType: contentType},
	)

	if err != nil {
		cblog.Error("Failed to upload object from reader:", err)
		return minio.UploadInfo{}, err
	}

	cblog.Infof("Successfully uploaded %s of size %d to bucket %s", objectName, info.Size, bucketName)

	uploadInfo := minio.UploadInfo{
		Bucket:       info.Bucket,
		Key:          info.Key,
		ETag:         info.ETag,
		Size:         info.Size,
		LastModified: info.LastModified,
		Location:     info.Location,
		VersionID:    info.VersionID,
	}

	return uploadInfo, nil
}

// CopyS3Object copies an object within S3
func CopyS3Object(connectionName string, srcBucket string, srcObject string, dstBucket string, dstObject string) (minio.ObjectInfo, error) {
	cblog.Info("call CopyS3Object()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", srcBucket)
	if err != nil {
		return minio.ObjectInfo{}, err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return minio.ObjectInfo{}, err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return minio.ObjectInfo{}, err
	}

	ctx := context.Background()
	srcOpts := minio.CopySrcOptions{
		Bucket: srcBucket,
		Object: srcObject,
	}

	dstOpts := minio.CopyDestOptions{
		Bucket: dstBucket,
		Object: dstObject,
	}

	_, err = client.CopyObject(ctx, dstOpts, srcOpts)
	if err != nil {
		cblog.Error("Failed to copy object:", err)
		return minio.ObjectInfo{}, err
	}

	objInfo, err := client.StatObject(ctx, dstBucket, dstObject, minio.StatObjectOptions{})
	if err != nil {
		cblog.Error("Failed to get copied object info:", err)
		return minio.ObjectInfo{}, err
	}

	return objInfo, nil
}

// CompletePart represents a part to be committed in CompleteMultipartUpload
type CompletePart struct {
	PartNumber int
	ETag       string
}

// DeleteResult represents the result of deleting an object
type DeleteResult struct {
	Key     string
	Success bool
	Error   string
}

// InitiateMultipartUpload initiates a multipart upload and returns upload ID
func InitiateMultipartUpload(connectionName string, bucketName string, objectName string) (string, error) {
	cblog.Info("call InitiateMultipartUpload()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return "", err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return "", err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return "", err
	}

	ctx := context.Background()

	// Use Core API to initiate multipart upload
	core := minio.Core{Client: client}
	uploadID, err := core.NewMultipartUpload(ctx, bucketName, objectName, minio.PutObjectOptions{})
	if err != nil {
		cblog.Error("Failed to initiate multipart upload:", err)
		return "", err
	}

	return uploadID, nil
}

// UploadPart uploads a part in a multipart upload
func UploadPart(connectionName string, bucketName string, objectName string, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	cblog.Info("call UploadPart()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return "", err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return "", err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return "", err
	}

	ctx := context.Background()

	core := minio.Core{Client: client}
	part, err := core.PutObjectPart(ctx, bucketName, objectName, uploadID, partNumber, reader, size, minio.PutObjectPartOptions{})
	if err != nil {
		cblog.Error("Failed to upload part:", err)
		return "", err
	}

	return part.ETag, nil
}

// CompleteMultipartUpload completes a multipart upload
func CompleteMultipartUpload(connectionName string, bucketName string, objectName string, uploadID string, parts []CompletePart) (string, string, error) {
	cblog.Info("call CompleteMultipartUpload()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return "", "", err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return "", "", err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return "", "", err
	}

	ctx := context.Background()
	var completeParts []minio.CompletePart
	for _, part := range parts {
		completeParts = append(completeParts, minio.CompletePart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
	}

	// Use Core API to complete multipart upload
	core := minio.Core{Client: client}
	uploadInfo, err := core.CompleteMultipartUpload(ctx, bucketName, objectName, uploadID, completeParts, minio.PutObjectOptions{})
	if err != nil {
		cblog.Error("Failed to complete multipart upload:", err)
		return "", "", err
	}

	location := fmt.Sprintf("/%s/%s", bucketName, objectName)
	return location, uploadInfo.ETag, nil
}

// AbortMultipartUpload aborts a multipart upload
func AbortMultipartUpload(connectionName string, bucketName string, objectName string, uploadID string) error {
	cblog.Info("call AbortMultipartUpload()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Use Core API to abort multipart upload
	core := minio.Core{Client: client}
	err = core.AbortMultipartUpload(ctx, bucketName, objectName, uploadID)
	if err != nil {
		cblog.Error("Failed to abort multipart upload:", err)
		return err
	}

	return nil
}

// DeleteMultipleObjects deletes multiple objects from a bucket
func DeleteMultipleObjects(connectionName string, bucketName string, objectNames []string) ([]DeleteResult, error) {
	cblog.Info("call DeleteMultipleObjects()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return nil, err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return nil, err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	objectsCh := make(chan minio.ObjectInfo)

	go func() {
		defer close(objectsCh)
		for _, objectName := range objectNames {
			objectsCh <- minio.ObjectInfo{
				Key: objectName,
			}
		}
	}()

	results := []DeleteResult{}
	for err := range client.RemoveObjects(ctx, bucketName, objectsCh, minio.RemoveObjectsOptions{}) {
		result := DeleteResult{
			Key:     err.ObjectName,
			Success: false,
			Error:   err.Err.Error(),
		}
		results = append(results, result)
	}

	for _, objectName := range objectNames {
		found := false
		for _, result := range results {
			if result.Key == objectName {
				found = true
				break
			}
		}
		if !found {
			results = append(results, DeleteResult{
				Key:     objectName,
				Success: true,
			})
		}
	}

	return results, nil
}

// ListMultipartUploads lists all multipart uploads in progress
func ListMultipartUploads(connectionName string, bucketName string, prefix string) ([]minio.ObjectMultipartInfo, error) {
	cblog.Info("call ListMultipartUploads()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return nil, err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return nil, err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	var uploads []minio.ObjectMultipartInfo

	multipartInfoCh := client.ListIncompleteUploads(ctx, bucketName, prefix, true)
	for multipartInfo := range multipartInfoCh {
		if multipartInfo.Err != nil {
			cblog.Error("Error listing multipart uploads:", multipartInfo.Err)
			continue
		}
		uploads = append(uploads, multipartInfo)
	}

	return uploads, nil
}

func GetS3PresignedURLWithHeaders(connectionName string, bucketName string, objectName string, method string, expiresSeconds int64, headers map[string][]string) (string, error) {
	cblog.Info("call GetS3PresignedURLWithHeaders()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return "", err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return "", err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	expiresDuration := time.Duration(expiresSeconds) * time.Second

	switch method {
	case "GET":
		// Convert headers map[string][]string to url.Values for GET
		reqParams := make(url.Values)
		for key, values := range headers {
			for _, value := range values {
				reqParams.Add(key, value)
			}
		}
		u, err := client.PresignedGetObject(ctx, bucketName, objectName, expiresDuration, reqParams)
		if err != nil {
			return "", err
		}
		return u.String(), nil

	case "PUT":
		// For PUT requests, we need to use PresignedPutObject
		u, err := client.PresignedPutObject(ctx, bucketName, objectName, expiresDuration)
		if err != nil {
			return "", err
		}

		// For PUT with specific content-type, we need to add it to the URL
		if contentTypes, ok := headers["Content-Type"]; ok && len(contentTypes) > 0 {
			parsedURL, err := url.Parse(u.String())
			if err != nil {
				return "", err
			}
			q := parsedURL.Query()
			q.Set("Content-Type", contentTypes[0])
			parsedURL.RawQuery = q.Encode()
			return parsedURL.String(), nil
		}

		return u.String(), nil

	default:
		return "", fmt.Errorf("Unsupported method: %s", method)
	}
}

func SetS3BucketCORS(connectionName string, bucketName string, allowedOrigins []string, allowedMethods []string, allowedHeaders []string, exposeHeaders []string, maxAgeSeconds int) (bool, error) {
	cblog.Info("call SetS3BucketCORS()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return false, err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return false, err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return false, err
	}

	ctx := context.Background()

	// Create CORS configuration using cors.Config with correct field names
	corsConfig := &cors.Config{
		CORSRules: []cors.Rule{
			{
				AllowedOrigin: allowedOrigins,
				AllowedMethod: allowedMethods,
				AllowedHeader: allowedHeaders,
				ExposeHeader:  exposeHeaders,
				MaxAgeSeconds: maxAgeSeconds,
			},
		},
	}

	// Set bucket CORS using Core API
	core := minio.Core{Client: client}
	err = core.SetBucketCors(ctx, bucketName, corsConfig)
	if err != nil {
		cblog.Error("Failed to set bucket CORS:", err)
		return false, err
	}

	cblog.Infof("Successfully set CORS for bucket %s", bucketName)
	return true, nil
}

// GetS3BucketCORS gets CORS configuration for a bucket
func GetS3BucketCORS(connectionName string, bucketName string) (*cors.Config, error) {
	cblog.Info("call GetS3BucketCORS()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return nil, err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return nil, err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	// Get bucket CORS using Core API
	core := minio.Core{Client: client}
	corsConfig, err := core.GetBucketCors(ctx, bucketName)
	if err != nil {
		// Check if CORS is not set
		if strings.Contains(err.Error(), "NoSuchCORSConfiguration") {
			return nil, fmt.Errorf("CORS configuration not found for bucket %s", bucketName)
		}
		cblog.Error("Failed to get bucket CORS:", err)
		return nil, err
	}

	return corsConfig, nil
}

func DeleteS3BucketCORS(connectionName string, bucketName string) (bool, error) {
	cblog.Info("call DeleteS3BucketCORS()")

	var iidInfo S3BucketIIDInfo
	err := infostore.GetByConditions(&iidInfo, "connection_name", connectionName, "name_id", bucketName)
	if err != nil {
		return false, err
	}

	connInfo, err := GetS3ConnectionInfo(connectionName)
	if err != nil {
		return false, err
	}

	client, err := NewS3Client(connInfo)
	if err != nil {
		return false, err
	}

	ctx := context.Background()

	// Delete bucket CORS by setting empty CORS configuration
	core := minio.Core{Client: client}
	emptyCORS := &cors.Config{
		CORSRules: []cors.Rule{},
	}
	err = core.SetBucketCors(ctx, bucketName, emptyCORS)
	if err != nil {
		cblog.Error("Failed to delete bucket CORS:", err)
		return false, err
	}

	cblog.Infof("Successfully deleted CORS for bucket %s", bucketName)
	return true, nil
}
