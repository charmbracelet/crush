// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package anthropic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"github.com/anthropics/anthropic-sdk-go/internal/apiform"
	"github.com/anthropics/anthropic-sdk-go/internal/apijson"
	"github.com/anthropics/anthropic-sdk-go/internal/apiquery"
	"github.com/anthropics/anthropic-sdk-go/internal/requestconfig"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/pagination"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/packages/respjson"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

// BetaFileService contains methods and other services that help with interacting
// with the anthropic API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewBetaFileService] method instead.
type BetaFileService struct {
	Options []option.RequestOption
}

// NewBetaFileService generates a new service that applies the given options to
// each request. These options are applied after the parent client's options (if
// there is one), and before any request-specific options.
func NewBetaFileService(opts ...option.RequestOption) (r BetaFileService) {
	r = BetaFileService{}
	r.Options = opts
	return
}

// List Files
func (r *BetaFileService) List(ctx context.Context, params BetaFileListParams, opts ...option.RequestOption) (res *pagination.Page[FileMetadata], err error) {
	var raw *http.Response
	for _, v := range params.Betas {
		opts = append(opts, option.WithHeaderAdd("anthropic-beta", fmt.Sprintf("%s", v)))
	}
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("anthropic-beta", "files-api-2025-04-14"), option.WithResponseInto(&raw)}, opts...)
	path := "v1/files?beta=true"
	cfg, err := requestconfig.NewRequestConfig(ctx, http.MethodGet, path, params, &res, opts...)
	if err != nil {
		return nil, err
	}
	err = cfg.Execute()
	if err != nil {
		return nil, err
	}
	res.SetPageConfig(cfg, raw)
	return res, nil
}

// List Files
func (r *BetaFileService) ListAutoPaging(ctx context.Context, params BetaFileListParams, opts ...option.RequestOption) *pagination.PageAutoPager[FileMetadata] {
	return pagination.NewPageAutoPager(r.List(ctx, params, opts...))
}

// Delete File
func (r *BetaFileService) Delete(ctx context.Context, fileID string, body BetaFileDeleteParams, opts ...option.RequestOption) (res *DeletedFile, err error) {
	for _, v := range body.Betas {
		opts = append(opts, option.WithHeaderAdd("anthropic-beta", fmt.Sprintf("%s", v)))
	}
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("anthropic-beta", "files-api-2025-04-14")}, opts...)
	if fileID == "" {
		err = errors.New("missing required file_id parameter")
		return
	}
	path := fmt.Sprintf("v1/files/%s?beta=true", fileID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodDelete, path, nil, &res, opts...)
	return
}

// Download File
func (r *BetaFileService) Download(ctx context.Context, fileID string, query BetaFileDownloadParams, opts ...option.RequestOption) (res *http.Response, err error) {
	for _, v := range query.Betas {
		opts = append(opts, option.WithHeaderAdd("anthropic-beta", fmt.Sprintf("%s", v)))
	}
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("anthropic-beta", "files-api-2025-04-14"), option.WithHeader("Accept", "application/binary")}, opts...)
	if fileID == "" {
		err = errors.New("missing required file_id parameter")
		return
	}
	path := fmt.Sprintf("v1/files/%s/content?beta=true", fileID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Get File Metadata
func (r *BetaFileService) GetMetadata(ctx context.Context, fileID string, query BetaFileGetMetadataParams, opts ...option.RequestOption) (res *FileMetadata, err error) {
	for _, v := range query.Betas {
		opts = append(opts, option.WithHeaderAdd("anthropic-beta", fmt.Sprintf("%s", v)))
	}
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("anthropic-beta", "files-api-2025-04-14")}, opts...)
	if fileID == "" {
		err = errors.New("missing required file_id parameter")
		return
	}
	path := fmt.Sprintf("v1/files/%s?beta=true", fileID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, nil, &res, opts...)
	return
}

// Upload File
func (r *BetaFileService) Upload(ctx context.Context, params BetaFileUploadParams, opts ...option.RequestOption) (res *FileMetadata, err error) {
	for _, v := range params.Betas {
		opts = append(opts, option.WithHeaderAdd("anthropic-beta", fmt.Sprintf("%s", v)))
	}
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("anthropic-beta", "files-api-2025-04-14")}, opts...)
	path := "v1/files?beta=true"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, params, &res, opts...)
	return
}

type DeletedFile struct {
	// ID of the deleted file.
	ID string `json:"id,required"`
	// Deleted object type.
	//
	// For file deletion, this is always `"file_deleted"`.
	//
	// Any of "file_deleted".
	Type DeletedFileType `json:"type"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r DeletedFile) RawJSON() string { return r.JSON.raw }
func (r *DeletedFile) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Deleted object type.
//
// For file deletion, this is always `"file_deleted"`.
type DeletedFileType string

const (
	DeletedFileTypeFileDeleted DeletedFileType = "file_deleted"
)

type FileMetadata struct {
	// Unique object identifier.
	//
	// The format and length of IDs may change over time.
	ID string `json:"id,required"`
	// RFC 3339 datetime string representing when the file was created.
	CreatedAt time.Time `json:"created_at,required" format:"date-time"`
	// Original filename of the uploaded file.
	Filename string `json:"filename,required"`
	// MIME type of the file.
	MimeType string `json:"mime_type,required"`
	// Size of the file in bytes.
	SizeBytes int64 `json:"size_bytes,required"`
	// Object type.
	//
	// For files, this is always `"file"`.
	Type constant.File `json:"type,required"`
	// Whether the file can be downloaded.
	Downloadable bool `json:"downloadable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID           respjson.Field
		CreatedAt    respjson.Field
		Filename     respjson.Field
		MimeType     respjson.Field
		SizeBytes    respjson.Field
		Type         respjson.Field
		Downloadable respjson.Field
		ExtraFields  map[string]respjson.Field
		raw          string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileMetadata) RawJSON() string { return r.JSON.raw }
func (r *FileMetadata) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaFileListParams struct {
	// ID of the object to use as a cursor for pagination. When provided, returns the
	// page of results immediately after this object.
	AfterID param.Opt[string] `query:"after_id,omitzero" json:"-"`
	// ID of the object to use as a cursor for pagination. When provided, returns the
	// page of results immediately before this object.
	BeforeID param.Opt[string] `query:"before_id,omitzero" json:"-"`
	// Number of items to return per page.
	//
	// Defaults to `20`. Ranges from `1` to `1000`.
	Limit param.Opt[int64] `query:"limit,omitzero" json:"-"`
	// Optional header to specify the beta version(s) you want to use.
	Betas []AnthropicBeta `header:"anthropic-beta,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [BetaFileListParams]'s query parameters as `url.Values`.
func (r BetaFileListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatComma,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

type BetaFileDeleteParams struct {
	// Optional header to specify the beta version(s) you want to use.
	Betas []AnthropicBeta `header:"anthropic-beta,omitzero" json:"-"`
	paramObj
}

type BetaFileDownloadParams struct {
	// Optional header to specify the beta version(s) you want to use.
	Betas []AnthropicBeta `header:"anthropic-beta,omitzero" json:"-"`
	paramObj
}

type BetaFileGetMetadataParams struct {
	// Optional header to specify the beta version(s) you want to use.
	Betas []AnthropicBeta `header:"anthropic-beta,omitzero" json:"-"`
	paramObj
}

type BetaFileUploadParams struct {
	// The file to upload
	File io.Reader `json:"file,omitzero,required" format:"binary"`
	// Optional header to specify the beta version(s) you want to use.
	Betas []AnthropicBeta `header:"anthropic-beta,omitzero" json:"-"`
	paramObj
}

func (r BetaFileUploadParams) MarshalMultipart() (data []byte, contentType string, err error) {
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	err = apiform.MarshalRoot(r, writer)
	if err == nil {
		err = apiform.WriteExtras(writer, r.ExtraFields())
	}
	if err != nil {
		writer.Close()
		return nil, "", err
	}
	err = writer.Close()
	if err != nil {
		return nil, "", err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}
