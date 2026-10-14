package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/google/uuid"
)

// ImgBed stores immutable remote IDs in FileStorageEntity.Source. Blog names
// and directories remain virtual, so moving them never invalidates file URLs.
type ImgBedProvider struct{ client *http.Client }

func NewImgBedProvider() IStorageProvider {
	return &ImgBedProvider{client: &http.Client{
		Timeout: 120 * time.Second,
		// Never follow an upload/read redirect with our API credentials.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// ValidateImgBedPolicy is shared by create/update validation and the driver.
func ValidateImgBedPolicy(policy *model.StoragePolicy) error {
	if policy == nil {
		return errors.New("图床策略不能为空")
	}
	u, err := url.Parse(policy.Server)
	if err != nil || u == nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" {
		return errors.New("图床地址必须是没有路径、凭据或查询参数的站点地址")
	}
	local := strings.EqualFold(u.Hostname(), "localhost")
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		local = ip.IsLoopback()
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return errors.New("图床地址必须使用 HTTPS；本机测试可用 HTTP")
	}
	if strings.TrimSpace(policy.SecretKey) == "" || policy.SecretKey == constant.SecretValueMask || strings.IndexFunc(policy.SecretKey, unicode.IsSpace) >= 0 {
		return errors.New("请填写图床 API Token")
	}
	if policy.IsPrivate {
		return errors.New("图床策略需要公开文件读取，不支持私有读写")
	}
	if !validImgBedKey(strings.Trim(policy.BasePath, "/")) {
		return errors.New("请填写安全且非空的图床目录，例如 blog/articles")
	}
	return nil
}

func validImgBedKey(key string) bool {
	// Match ImgBed's sanitizeUploadFolder without silently rewriting a saved
	// root. Otherwise an upload can succeed remotely but return an unusable ID.
	if key == "" || strings.ContainsAny(key, "\\:*?\"'<>| ()[]{}#%^`~;@&=+$,") || strings.IndexFunc(key, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r) || r == '\uFEFF'
	}) >= 0 {
		return false
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || strings.Contains(part, "..") {
			return false
		}
	}
	return true
}

func (p *ImgBedProvider) fileURL(policy *model.StoragePolicy, prefix, source string) (string, error) {
	if err := ValidateImgBedPolicy(policy); err != nil {
		return "", err
	}
	if !validImgBedKey(source) || !strings.HasPrefix(source, strings.Trim(policy.BasePath, "/")+"/") {
		return "", errors.New("无效的图床文件标识")
	}
	parts := strings.Split(source, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.TrimRight(policy.Server, "/") + prefix + strings.Join(parts, "/"), nil
}

func (p *ImgBedProvider) request(ctx context.Context, policy *model.StoragePolicy, method, endpoint string, body io.Reader, contentType string, length int64) (*http.Response, error) {
	if err := ValidateImgBedPolicy(policy); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, errors.New("无法创建图床请求")
	}
	req.Header.Set("Authorization", "Bearer "+policy.SecretKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if length >= 0 {
		req.ContentLength = length
	}
	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Do not retain upstream error strings: they can contain credentials/URLs.
		return nil, errors.New("图床连接失败，请检查地址和网络")
	}
	return resp, nil
}

func imgbedStatus(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("图床请求失败: HTTP %d", resp.StatusCode)
	}
	return nil
}

var imgbedExtension = regexp.MustCompile(`^\.[a-zA-Z0-9]{1,12}$`)

func (p *ImgBedProvider) Upload(ctx context.Context, file io.Reader, policy *model.StoragePolicy, virtualPath string) (*UploadResult, error) {
	if err := ValidateImgBedPolicy(policy); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Spool the multipart body to disk instead of buffering arbitrary files in RAM.
	temp, err := os.CreateTemp("", "anheyu-imgbed-upload-*")
	if err != nil {
		return nil, errors.New("无法创建图床上传暂存文件")
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	writer := multipart.NewWriter(temp)
	ext := strings.ToLower(path.Ext(virtualPath))
	if !imgbedExtension.MatchString(ext) {
		ext = ".bin"
	}
	filename := uuid.NewString() + ext
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	h := make(textproto.MIMEHeader)
	// Node/Undici requires quoted multipart names, matching browser FormData.
	// The filename is generated from a UUID and a restricted extension.
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(h)
	if err != nil {
		return nil, errors.New("无法准备图床上传表单")
	}
	input := io.Reader(&imgbedContextReader{ctx: ctx, reader: file})
	if policy.MaxSize > 0 && policy.MaxSize < int64(^uint64(0)>>1) {
		input = io.LimitReader(input, policy.MaxSize+1)
	}
	size, err := io.Copy(part, input)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("读取上传文件失败")
	}
	if policy.MaxSize > 0 && size > policy.MaxSize {
		return nil, errors.New("文件大小超出图床策略限制")
	}
	if err := writer.Close(); err != nil {
		return nil, errors.New("无法完成图床上传表单")
	}
	length, err := temp.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, errors.New("无法读取图床上传长度")
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return nil, errors.New("无法读取图床上传暂存文件")
	}
	u, _ := url.Parse(strings.TrimRight(policy.Server, "/") + "/upload")
	q := u.Query()
	q.Set("uploadFolder", strings.Trim(policy.BasePath, "/"))
	q.Set("uploadNameType", "origin")
	q.Set("returnFormat", "default")
	q.Set("autoRetry", "false")
	q.Set("uploadChannel", policy.Settings.GetString("upload_channel", "s3"))
	if channel := policy.Settings.GetString("channel_name", ""); channel != "" {
		q.Set("channelName", channel)
	}
	u.RawQuery = q.Encode()
	resp, err := p.request(ctx, policy, http.MethodPost, u.String(), temp, writer.FormDataContentType(), length)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := imgbedStatus(resp); err != nil {
		return nil, err
	}
	var result []struct {
		Src string `json:"src"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&result); err != nil || len(result) != 1 {
		return nil, errors.New("图床上传响应格式错误")
	}
	link, err := url.Parse(result[0].Src)
	if err != nil || link.IsAbs() || link.Host != "" || link.RawQuery != "" || link.Fragment != "" || !strings.HasPrefix(link.Path, "/file/") {
		return nil, errors.New("图床返回了无效的文件地址")
	}
	source := strings.TrimPrefix(link.Path, "/file/")
	if _, err := p.fileURL(policy, "/file/", source); err != nil {
		return nil, err
	}
	return &UploadResult{Source: source, Size: size, MimeType: mimeType}, nil
}

type imgbedContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *imgbedContextReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(b)
}

func (p *ImgBedProvider) GetDownloadURL(ctx context.Context, policy *model.StoragePolicy, source string, options DownloadURLOptions) (string, error) {
	// Public URLs never contain the management token. Private access is rejected.
	return p.fileURL(policy, "/file/", source)
}

func (p *ImgBedProvider) Get(ctx context.Context, policy *model.StoragePolicy, source string) (io.ReadCloser, error) {
	u, err := p.fileURL(policy, "/file/", source)
	if err != nil {
		return nil, err
	}
	resp, err := p.request(ctx, policy, http.MethodGet, u, nil, "", 0)
	if err != nil {
		return nil, err
	}
	if err := imgbedStatus(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}
	return resp.Body, nil
}

func (p *ImgBedProvider) Stream(ctx context.Context, policy *model.StoragePolicy, source string, w io.Writer) error {
	r, err := p.Get(ctx, policy, source)
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(w, r)
	return err
}

func (p *ImgBedProvider) IsExist(ctx context.Context, policy *model.StoragePolicy, source string) (bool, error) {
	u, err := p.fileURL(policy, "/file/", source)
	if err != nil {
		return false, err
	}
	resp, err := p.request(ctx, policy, http.MethodHead, u, nil, "", 0)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return resp.StatusCode >= 200 && resp.StatusCode < 300, imgbedStatus(resp)
}

func (p *ImgBedProvider) Delete(ctx context.Context, policy *model.StoragePolicy, sources []string) error {
	for _, source := range sources {
		u, err := p.fileURL(policy, "/api/manage/delete/", source)
		if err != nil {
			return err
		}
		resp, err := p.request(ctx, policy, http.MethodDelete, u, nil, "", 0)
		if err != nil {
			return err
		}
		if err := imgbedStatus(resp); err != nil {
			resp.Body.Close()
			return err
		}
		var result struct {
			Success bool `json:"success"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&result)
		resp.Body.Close()
		if err != nil || !result.Success {
			return errors.New("图床删除失败，文件记录已保留")
		}
	}
	return nil
}

// Directories and renames are blog database operations; the remote object ID
// and upload folder deliberately remain unchanged.
func (p *ImgBedProvider) CreateDirectory(context.Context, *model.StoragePolicy, string) error {
	return nil
}
func (p *ImgBedProvider) DeleteDirectory(context.Context, *model.StoragePolicy, string) error {
	return nil
}
func (p *ImgBedProvider) Rename(context.Context, *model.StoragePolicy, string, string) error {
	return nil
}
func (p *ImgBedProvider) List(context.Context, *model.StoragePolicy, string) ([]FileInfo, error) {
	return nil, ErrFeatureNotSupported
}
func (p *ImgBedProvider) GetThumbnail(context.Context, *model.StoragePolicy, string, string) (*ThumbnailResult, error) {
	return nil, ErrFeatureNotSupported
}
