package openai

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/openai/openai-go/option"
)

func mkPollingOptions(pollIntervalMs int) []option.RequestOption {
	options := []option.RequestOption{option.WithHeader("X-Stainless-Poll-Helper", "true")}
	if pollIntervalMs > 0 {
		options = append(options, option.WithHeader("X-Stainless-Poll-Interval", fmt.Sprintf("%d", pollIntervalMs)))
	}
	return options
}

func getPollInterval(raw *http.Response) (ms int) {
	if ms, err := strconv.Atoi(raw.Header.Get("openai-poll-after-ms")); err == nil {
		return ms
	}
	return 1000
}

// PollStatus waits until a VectorStoreFile is no longer in an incomplete state and returns it.
// Pass 0 as pollIntervalMs to use the default polling interval of 1 second.
func (r *VectorStoreFileService) PollStatus(ctx context.Context, vectorStoreID string, fileID string, pollIntervalMs int, opts ...option.RequestOption) (*VectorStoreFile, error) {
	var raw *http.Response
	opts = append(opts, mkPollingOptions(pollIntervalMs)...)
	opts = append(opts, option.WithResponseInto(&raw))
	for {
		file, err := r.Get(ctx, fileID, vectorStoreID, opts...)
		if err != nil {
			return nil, fmt.Errorf("vector store file poll: received %w", err)
		}

		switch file.Status {
		case VectorStoreFileStatusInProgress:
			if pollIntervalMs <= 0 {
				pollIntervalMs = getPollInterval(raw)
			}
			time.Sleep(time.Duration(pollIntervalMs) * time.Millisecond)
		case VectorStoreFileStatusCancelled,
			VectorStoreFileStatusCompleted,
			VectorStoreFileStatusFailed:
			return file, nil
		default:
			return nil, fmt.Errorf("invalid vector store file status during polling: received %s", file.Status)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:

		}
	}
}

// PollStatus waits until a BetaVectorStoreFileBatch is no longer in an incomplete state and returns it.
// Pass 0 as pollIntervalMs to use the default polling interval of 1 second.
func (r *VectorStoreFileBatchService) PollStatus(ctx context.Context, vectorStoreID string, batchID string, pollIntervalMs int, opts ...option.RequestOption) (*VectorStoreFileBatch, error) {
	var raw *http.Response
	opts = append(opts, option.WithResponseInto(&raw))
	opts = append(opts, mkPollingOptions(pollIntervalMs)...)
	for {
		batch, err := r.Get(ctx, batchID, vectorStoreID, opts...)
		if err != nil {
			return nil, fmt.Errorf("vector store file batch poll: received %w", err)
		}

		switch batch.Status {
		case VectorStoreFileBatchStatusInProgress:
			if pollIntervalMs <= 0 {
				pollIntervalMs = getPollInterval(raw)
			}
			time.Sleep(time.Duration(pollIntervalMs) * time.Millisecond)
		case VectorStoreFileBatchStatusCancelled,
			VectorStoreFileBatchStatusCompleted,
			VectorStoreFileBatchStatusFailed:
			return batch, nil
		default:
			return nil, fmt.Errorf("invalid vector store file batch status during polling: received %s", batch.Status)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}
}
