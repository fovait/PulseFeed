package message

import (
	"PulseFeed/internal/account"
	"context"
	"errors"
	"strings"
)

type Service struct {
	repo        *Repository
	accountRepo *account.AccountRepository
}

func NewService(repo *Repository, accountRepo *account.AccountRepository) *Service {
	return &Service{
		repo:        repo,
		accountRepo: accountRepo,
	}
}

func (s *Service) Send(ctx context.Context, fromID uint, req SendRequest) (*Message, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("message service is not initialized")
	}

	content := strings.TrimSpace(req.Content)

	if fromID == 0 || req.ToID == 0 {
		return nil, errors.New("from_id and to_id are required")
	}
	if fromID == req.ToID {
		return nil, errors.New("can not send message to yourself")
	}
	if content == "" {
		return nil, errors.New("content is required")
	}

	// 检查接收者是否存在。
	// 如果你暂时不想引入 accountRepo，可以允许 accountRepo 为 nil。
	if s.accountRepo != nil {
		if _, err := s.accountRepo.FindByID(ctx, req.ToID); err != nil {
			return nil, err
		}
	}

	msg := &Message{
		FromID:  fromID,
		ToID:    req.ToID,
		Content: content,
	}

	if err := s.repo.Create(ctx, msg); err != nil {
		return nil, err
	}

	return msg, nil
}

func (s *Service) List(ctx context.Context, userID uint, req ListRequest) (ListResponse, error) {
	if s == nil || s.repo == nil {
		return ListResponse{}, errors.New("message service is not initialized")
	}
	if userID == 0 || req.PeerID == 0 {
		return ListResponse{}, errors.New("user_id and peer_id are required")
	}

	limit := normalizeLimit(req.Limit)

	// 多查一条，用来判断 has_more。
	queryLimit := limit + 1

	msgs, err := s.repo.ListConversation(ctx, userID, req.PeerID, queryLimit, req.BeforeID)
	if err != nil {
		return ListResponse{}, err
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}

	var nextBeforeID uint
	if len(msgs) > 0 {
		// repo 返回的是 id desc，所以最后一条是这一页最旧的消息。
		nextBeforeID = msgs[len(msgs)-1].ID
	}

	// 聊天窗口通常希望从旧到新展示，所以这里反转一下。
	reverseMessages(msgs)

	// 查询聊天记录后，把对方发给我的未读消息标记为已读。
	_ = s.repo.MarkReadFromPeer(ctx, userID, req.PeerID)

	if msgs == nil {
		msgs = []Message{}
	}

	return ListResponse{
		Messages:     msgs,
		NextBeforeID: nextBeforeID,
		HasMore:      hasMore,
	}, nil
}

func (s *Service) ListConversations(ctx context.Context, userID uint, req ListConversationsRequest) (ListConversationsResponse, error) {
	if s == nil || s.repo == nil {
		return ListConversationsResponse{}, errors.New("message service is not initialized")
	}
	if userID == 0 {
		return ListConversationsResponse{}, errors.New("user_id is required")
	}

	conversations, err := s.repo.ListConversations(ctx, userID, normalizeConversationLimit(req.Limit))
	if err != nil {
		return ListConversationsResponse{}, err
	}

	// 批量查 peer 的 username，避免 N+1。
	if s.accountRepo != nil && len(conversations) > 0 {
		peerIDs := make([]uint, 0, len(conversations))
		for _, c := range conversations {
			if c.PeerID > 0 {
				peerIDs = append(peerIDs, c.PeerID)
			}
		}
		accounts, err := s.accountRepo.FindByIDs(ctx, peerIDs)
		if err == nil {
			for i := range conversations {
				if a, ok := accounts[conversations[i].PeerID]; ok {
					conversations[i].PeerUsername = a.Username
				}
			}
		}
	}

	var unreadCount int64
	for i := range conversations {
		unreadCount += conversations[i].UnreadCount
	}
	if conversations == nil {
		conversations = []Conversation{}
	}

	return ListConversationsResponse{
		Conversations: conversations,
		UnreadCount:   unreadCount,
	}, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func normalizeConversationLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func reverseMessages(msgs []Message) {
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
}
