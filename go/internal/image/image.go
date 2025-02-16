package image

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"

	"r2img/internal/config"

	"github.com/chai2010/webp"
)

// ConvertToWebp 将 PNG 或 JPEG 图像转换为 WebP 格式。
func ConvertToWebp(fileBytes []byte, fileExt string, quality float32) ([]byte, error) {
	imgReader := bytes.NewReader(fileBytes)
	var img image.Image
	var err error

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

	outputBuffer := bytes.NewBuffer(nil)
	if err := webp.Encode(outputBuffer, img, &webp.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("WebP 编码失败: %v", err)
	}

	return outputBuffer.Bytes(), nil
}

// UploadToAPI 将 WebP 图像文件上传到远程 API。
func UploadToAPI(fileBytes []byte, cfg *config.Config) (string, error) {
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	fileName, err := generateUniqueFilename()
	if err != nil {
		return "", fmt.Errorf("生成文件名失败: %v", err)
	}

	partHeader := textproto.MIMEHeader{}
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s.webp"`, fileName))
	partHeader.Set("Content-Type", "image/webp")

	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return "", fmt.Errorf("创建表单字段失败: %v", err)
	}

	if _, err = part.Write(fileBytes); err != nil {
		return "", fmt.Errorf("写入文件内容失败: %v", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭 multipart writer 失败: %v", err)
	}

	req, err := http.NewRequest("POST", cfg.ApiSite+"/upload", buf)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("上传失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("上传失败，状态码: %d", resp.StatusCode)
	}

	return fileName + ".webp", nil
}

// generateUniqueFilename 生成唯一文件名
func generateUniqueFilename() (string, error) {
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)
	randomBytes := make([]byte, 18)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("生成随机字节失败: %v", err)
	}
	encoded := base64.URLEncoding.EncodeToString(randomBytes)
	encoded = strings.TrimRight(encoded, "=") // 去除末尾的 '=' 字符

	filename := fmt.Sprintf("%d%s", timestamp, encoded)
	return filename[:30], nil // 截取到30位
}

// HandleFileUpload 处理文件上传逻辑
func HandleFileUpload(file multipart.File, header *multipart.FileHeader, cfg *config.Config) (string, error) {
	fileName := header.Filename
	fileExt := strings.ToLower(filepath.Ext(fileName))
	if fileExt != ".png" && fileExt != ".jpg" && fileExt != ".jpeg" && fileExt != ".webp" {
		return "", fmt.Errorf("不支持的文件类型: %s", fileExt)
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %v", err)
	}

	if fileExt != ".webp" {
		fileBytes, err = ConvertToWebp(fileBytes, fileExt, cfg.Quality)
		if err != nil {
			return "", fmt.Errorf("图片转换失败: %v", err)
		}
	}

	uploadedFilename, err := UploadToAPI(fileBytes, cfg)
	if err != nil {
		return "", err // 直接返回错误，不再需要 http.Error
	}
	return uploadedFilename, nil
}
