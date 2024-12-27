package main

import (
	"fmt"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"go-jvm-heapdump-monitor/monitor"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

func main() {
	var dumpPath = getEnvDefault("OPS_APPLICATION_MEM_DUMP_PATH", "")
	var enable = getEnvDefault("OPS_APPLICATION_MEM_DUMP_ENABLE", "true")
	if enable != "true" {
		return
	}
	uploadFile(dumpPath)
}

func uploadFile(file string) {
	fmt.Printf("start upload ali yun oss, file: %s \n", file)
	var accessKeyId = getEnvDefault("OPS_ALI_KEY_ID", "")
	var accessKeySecret = getEnvDefault("OPS_ALI_KEY_SECRET", "")
	var bucketName = getEnvDefault("OPS_APPLICATION_MEM_DUMP_BUCKET", "")
	var endpoint = getEnvDefault("OPS_APPLICATION_OSS_ENDPOINT", "")
	var applicationName = getEnvDefault("OPS_APPLICATION_NAME", "")
	client, err := oss.New(endpoint, accessKeyId, accessKeySecret)
	if err != nil {
		log.Fatalf("create oss link failed: %v", err)
	}
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		log.Fatalf("get oss bucket failed: %v", err)
	}
	var filename = filepath.Base(file) //获取文件名
	// 设置存储类型为标准存储。
	var objectName = "k8s/application-mem-dump/" + applicationName + "/" + formatDateTime() + filename
	if err := uploadMultipart(bucket, objectName, file, int64(5*1024*1024)); err != nil {
		log.Fatalf("Failed to upload multipart: %v", err)
	}
	sendAlert(applicationName, bucketName, objectName)
}

func sendAlert(applicationName string, bucket string, memoryDumpPath string) {
	var robotId = getEnvDefault("OPS_APPLICATION_MONITOR_ROBOT_ID", "")
	param := &monitor.ParamCronTask{
		Text: struct {
			Content string `json:"content"`
		}{
			Content: "堆内存溢出告警 \n\n" +
				"应用名称:" + applicationName + "\n" +
				"IP: " + getHostIp() + "\n" +
				"内存快照OSS Bucket: " + bucket + "\n" +
				"内存快照位置: " + memoryDumpPath,
		},
		Msgtype: "text",
	}
	err := (&monitor.DingRobot{RobotId: robotId}).SendMessage(param)
	if err != nil {
		fmt.Println(err)
	}
}

func getEnvDefault(key, defaultVal string) string {
	val, ex := os.LookupEnv(key)
	if !ex {
		return defaultVal
	}
	return val
}

func getHostIp() string {
	addrList, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println("get current host ip err: ", err)
		return ""
	}
	var ip string
	for _, address := range addrList {
		if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ip = ipNet.IP.String()
				break
			}
		}
	}
	return ip
}

func formatDateTime() string {
	return time.Now().Format("20060102150405")
}

// 分片上传函数
func uploadMultipart(bucket *oss.Bucket, objectName, localFilename string, partSize int64) error {
	// 将本地文件分片
	chunks, err := oss.SplitFileByPartSize(localFilename, partSize)
	if err != nil {
		return fmt.Errorf("failed to split file into chunks: %w", err)
	}

	// 打开本地文件。
	file, err := os.Open(localFilename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// 步骤1：初始化一个分片上传事件。
	imur, err := bucket.InitiateMultipartUpload(objectName)
	if err != nil {
		return fmt.Errorf("failed to initiate multipart upload: %w", err)
	}

	// 步骤2：上传分片。
	var parts []oss.UploadPart
	for _, chunk := range chunks {
		part, err := bucket.UploadPart(imur, file, chunk.Size, chunk.Number)
		if err != nil {
			// 如果上传某个部分失败，尝试取消整个上传任务。
			if abortErr := bucket.AbortMultipartUpload(imur); abortErr != nil {
				log.Printf("Failed to abort multipart upload: %v", abortErr)
			}
			return fmt.Errorf("failed to upload part: %w", err)
		}
		parts = append(parts, part)
	}

	// 指定Object的读写权限为私有，默认为继承Bucket的读写权限。
	objectAcl := oss.ObjectACL(oss.ACLPrivate)

	// 步骤3：完成分片上传。
	_, err = bucket.CompleteMultipartUpload(imur, parts, objectAcl)
	if err != nil {
		// 如果完成上传失败，尝试取消上传。
		if abortErr := bucket.AbortMultipartUpload(imur); abortErr != nil {
			log.Printf("Failed to abort multipart upload: %v", abortErr)
		}
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	log.Printf("Multipart upload completed successfully.")
	return nil
}
