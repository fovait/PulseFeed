import { describe, expect, it, vi } from "vitest";

vi.mock("../client", () => ({
  apiClient: {
    request: vi.fn(),
    upload: vi.fn(),
  },
}));

import { apiClient } from "../client";
import { pulsefeedApi } from "../pulsefeed";

const request = vi.mocked(apiClient.request);
const upload = vi.mocked(apiClient.upload);

describe("pulsefeedApi", () => {
  it("uses backend request shapes for batch details and follow status", async () => {
    request.mockResolvedValueOnce({});
    request.mockResolvedValueOnce({});

    await pulsefeedApi.listVideoDetails([1, 2, 3]);
    await pulsefeedApi.isFollowing(9);

    expect(request).toHaveBeenNthCalledWith(1, "/video/listDetails", {
      auth: false,
      body: { ids: [1, 2, 3] },
    });
    expect(request).toHaveBeenNthCalledWith(2, "/social/isFollowed", {
      body: { vlogger_id: 9 },
    });
  });

  it("uses multipart upload endpoints for video and cover files", async () => {
    const formData = new FormData();
    upload.mockResolvedValueOnce({});
    upload.mockResolvedValueOnce({});

    await pulsefeedApi.uploadVideoFile(formData);
    await pulsefeedApi.uploadCoverFile(formData);

    expect(upload).toHaveBeenNthCalledWith(1, "/video/uploadVideo", formData);
    expect(upload).toHaveBeenNthCalledWith(2, "/video/uploadCover", formData);
  });
});
