package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"r2img/utils"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chai2010/webp"
)

type fileInfo struct {
	path  string
	size  int64
	atime time.Time
}

type config struct {
	ApiSite       string  `json:"api_site"`
	ApiKey        string  `json:"api_key"`
	AuthKey       string  `json:"auth_key"`
	Quality       float32 `json:"quality"`
	MaxFileSize   int64   `json:"max_file_size"`
	Port          int     `json:"port"`
	MaxCacheSize  int64   `json:"max_cache_size"`
	FreeCacheSize int64   `json:"free_cache_size"`
}

func main() {
	err := os.MkdirAll("./i", 0755) // 创建目录，并设置权限
	if err != nil {
		fmt.Println("Error creating directory:", err)
		return // 或者 panic(err)，根据你的错误处理策略
	}
	// 加载配置文件
	configPath := "config.json"
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Printf("加载配置文件失败: %v\n", err)
		return
	}

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		handleUpload(w, r, cfg)
	})
	http.HandleFunc("/i/", func(w http.ResponseWriter, r *http.Request) {
		handleImage(w, r, cfg)
	})

	log.Printf("服务启动，监听端口: %d\n", cfg.Port)
	http.ListenAndServe(":"+strconv.Itoa(cfg.Port), nil)
}

func cleanOldFiles(dir string, maxSize, freeSize int64) error {
	var files []fileInfo
	var totalSize int64 = 0

	// 遍历文件夹
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 忽略目录
		if info.IsDir() {
			return nil
		}

		atime, err := utils.GetFileAccessTime(path)
		if err != nil {
			log.Printf("读取文件时期失败: %v\n", err)
			return nil // 忽略错误，继续清理其他文件
		}
		files = append(files, fileInfo{path: path, size: info.Size(), atime: atime})
		totalSize += info.Size()
		return nil
	})
	if err != nil {
		log.Printf("遍历文件夹失败: %v\n", err)
		return err
	}

	// 如果总大小未超过限制，则不删除
	if totalSize < maxSize {
		return nil
	}

	// 按访问时间排序（最旧的在前）
	sort.Slice(files, func(i, j int) bool {
		return files[i].atime.Before(files[j].atime)
	})

	// 逐步删除最旧的文件，直到释放 FreeSize
	var deletedSize int64 = 0
	for _, file := range files {
		if deletedSize >= (totalSize - freeSize) {
			break
		}
		err := os.Remove(file.path)
		if err != nil {
			log.Printf("删除文件失败: %s, 错误: %v\n", file.path, err)
			continue
		}
		deletedSize += file.size
		log.Printf("成功删除文件: %s\n", file.path)
	}

	return nil
}

func handleImage(w http.ResponseWriter, r *http.Request, cfg *config) {
	// 获取 filename
	filename := strings.TrimPrefix(r.URL.Path, "/i/")

	// 检查 filename 是否为空
	if filename == "" {
		log.Println("请求的 filename 为空")
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 构造文件路径
	filePath := "./i/" + filename

	// 检查本地是否存在
	if _, err := os.Stat(filePath); err == nil {
		// 存在，直接返回
		http.ServeFile(w, r, filePath)
		return
	}

	// 清理旧文件
	err := cleanOldFiles("./i", cfg.MaxCacheSize*1024*1024, cfg.FreeCacheSize*1024*1024)
	if err != nil {
		log.Printf("清理旧文件失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 不存在，请求远程 URL
	remoteURL := cfg.ApiSite + "/i/" + filename
	req, err := http.NewRequest("GET", remoteURL, nil)
	if err != nil {
		log.Printf("创建请求失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("请求远程资源失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("远程资源返回状态码: %d\n", resp.StatusCode)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 读取响应内容
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取远程内容失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 保存到本地
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		log.Printf("保存文件失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 返回文件
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Write(data)
}

func loadConfig(path string) (*config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开配置文件失败: %v", err)
	}
	defer file.Close()

	var cfg config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("解码配置文件失败: %v", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("配置文件验证失败: %v", err)
	}

	return &cfg, nil
}

func validateConfig(cfg *config) error {
	if cfg.ApiSite == "" || cfg.ApiKey == "" || cfg.AuthKey == "" || cfg.Quality == 0 || cfg.MaxFileSize == 0 || cfg.Port == 0 || cfg.MaxCacheSize == 0 || cfg.FreeCacheSize == 0 {
		return fmt.Errorf("配置文件缺少必要字段")
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("无效的端口号: %d", cfg.Port)
	}

	if cfg.MaxFileSize <= 0 || cfg.MaxFileSize > 1024*1024 {
		return fmt.Errorf("最大文件大小过大或过小: %d", cfg.MaxFileSize)
	}

	return nil
}

func handleUpload(w http.ResponseWriter, r *http.Request, cfg *config) {
	// 检查认证
	auth := r.Header.Get("Authorization")
	if auth != "Bearer "+cfg.AuthKey {
		log.Printf("认证失败: 无效的授权头\n")
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 限制上传文件大小
	r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxFileSize*1024*1024)
	err := r.ParseMultipartForm(cfg.MaxFileSize * 1024 * 1024)
	if err != nil {
		log.Printf("解析多部分表单失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("获取上传文件失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// 检查文件类型
	fileName := header.Filename
	fileExt := strings.ToLower(filepath.Ext(fileName))
	if fileExt != ".png" && fileExt != ".jpg" && fileExt != ".jpeg" && fileExt != ".webp" {
		log.Printf("不支持的文件类型: %s\n", fileExt)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		log.Printf("读取文件失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 如果不是 webp，转化
	if fileExt != ".webp" {
		fileBytes, err = convertToWebp(fileBytes, fileExt, cfg.Quality)
		if err != nil {
			log.Printf("图片转换失败: %v\n", err)
			http.Error(w, "服务器内部错误", http.StatusInternalServerError)
			return
		}
	}

	// 上传 webp 文件
	uploadToApi(w, fileBytes, cfg)
}

func convertToWebp(fileBytes []byte, fileExt string, quality float32) ([]byte, error) {
	imgReader := bytes.NewReader(fileBytes)
	var img image.Image
	var err error

	// 解码图片
	switch fileExt {
	case ".png":
		img, err = png.Decode(imgReader)
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(imgReader)
	default:
		return nil, fmt.Errorf("不支持的文件格式: %s", fileExt)
	}
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %v", err)
	}

	// 创建缓冲区存储 WebP 数据
	outputBuffer := bytes.NewBuffer(nil)

	// 编码为 WebP
	if err := webp.Encode(outputBuffer, img, &webp.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("WebP 编码失败: %v", err)
	}

	return outputBuffer.Bytes(), nil
}

// 上传文件
func uploadToApi(w http.ResponseWriter, fileBytes []byte, cfg *config) {
	// 创建一个新的缓冲区，用于存储multipart内容
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	fileName, err := generateUniqueFilename()
	if err != nil {
		log.Printf("生成文件名失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 手动设置 Content-Type
	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s.webp"`, fileName))
	partHeader.Set("Content-Type", "image/webp")

	// 创建文件部分
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		log.Printf("创建表单字段失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 写入文件内容
	_, err = part.Write(fileBytes)
	if err != nil {
		log.Printf("写入文件内容失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 结束multipart写入
	if err := writer.Close(); err != nil {
		log.Printf("关闭multipart writer失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 创建 POST 请求
	req, err := http.NewRequest("POST", cfg.ApiSite+"/upload", buf)
	if err != nil {
		log.Printf("创建请求失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 设置 Authorization 头
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 执行请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("上传失败: %v\n", err)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("上传失败，状态码: %d\n", resp.StatusCode)
		http.Error(w, "服务器内部错误", http.StatusInternalServerError)
		return
	}

	// 上传成功
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fileName + ".webp"))
}

// 生成唯一文件名，长度 30，包含大小写字母和数字
func generateUniqueFilename() (string, error) {
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	// 增加随机字节数，以满足更长的文件名需求
	randomBytes := make([]byte, 18) // 原来是 12，现在增加到 18
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("生成随机数失败: %v", err)
	}

	encoded := base64.URLEncoding.EncodeToString(randomBytes)
	encoded = strings.TrimRight(encoded, "=")

	// 使用更长的 timestamp 部分, 避免取模后长度不够
	filename := fmt.Sprintf("%d%s", timestamp, encoded)
	return filename[:30], nil // 截取到 30 位
}
