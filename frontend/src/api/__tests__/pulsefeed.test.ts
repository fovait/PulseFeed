import { beforeEach, describe, expect, it, vi } from "vitest";

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
  beforeEach(() => {
    vi.clearAllMocks();
  });

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

  it("uses backend request shapes for account profile editing", async () => {
    request.mockResolvedValueOnce({ token: "next-token" });
    request.mockResolvedValueOnce({});
    request.mockResolvedValueOnce({});

    await pulsefeedApi.rename("next_name");
    await pulsefeedApi.updateProfile({ avatar_url: "https://cdn.test/avatar.png", bio: "hello" });
    await pulsefeedApi.changePassword("next_name", "old-pass", "new-pass");

    expect(request).toHaveBeenNthCalledWith(1, "/account/rename", {
      body: { new_username: "next_name" },
    });
    expect(request).toHaveBeenNthCalledWith(2, "/account/updateProfile", {
      body: { avatar_url: "https://cdn.test/avatar.png", bio: "hello" },
    });
    expect(request).toHaveBeenNthCalledWith(3, "/account/changePassword", {
      auth: false,
      body: { username: "next_name", old_password: "old-pass", new_password: "new-pass" },
    });
  });

  it("uses backend request shape for username account lookup", async () => {
    request.mockResolvedValueOnce({});

    await pulsefeedApi.findAccountByUsername("alice");

    expect(request).toHaveBeenCalledWith("/account/findByUsername", {
      auth: false,
      body: { username: "alice" },
    });
  });

  it("uses backend request shape for message conversations", async () => {
    request.mockResolvedValueOnce({});

    await pulsefeedApi.listMessageConversations(25);

    expect(request).toHaveBeenCalledWith("/message/conversations", {
      body: { limit: 25 },
    });
  });

  it("requests recommendations without debug payload by default", async () => {
    request.mockResolvedValueOnce({});

    await pulsefeedApi.recommend(8, "cursor-1");

    expect(request).toHaveBeenCalledWith("/feed/recommend", {
      body: { limit: 8, cursor: "cursor-1", debug: false },
    });
  });

  it("uses multipart upload endpoint for account avatars", async () => {
    const formData = new FormData();
    upload.mockResolvedValueOnce({});

    await pulsefeedApi.uploadAvatar(formData);

    expect(upload).toHaveBeenCalledWith("/account/uploadAvatar", formData);
  });
});
