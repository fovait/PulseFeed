package router

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"time"

	"PulseFeed/internal/account"
	"PulseFeed/internal/config"
	"PulseFeed/internal/event"
	"PulseFeed/internal/feed"
	"PulseFeed/internal/message"
	"PulseFeed/internal/middleware/jwt"
	"PulseFeed/internal/middleware/rabbitmq"
	"PulseFeed/internal/middleware/ratelimit"
	rediscache "PulseFeed/internal/middleware/redis"
	"PulseFeed/internal/moderation"
	"PulseFeed/internal/recommend"
	"PulseFeed/internal/social"
	"PulseFeed/internal/video"
	"PulseFeed/internal/worker"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetRouter 注册 HTTP 路由并构造后台 worker 句柄；worker 需由 main 调用 bg.Start(ctx) 启动。
func SetRouter(db *gorm.DB, cache *rediscache.Client, rmq *rabbitmq.RabbitMQ) (*gin.Engine, *BackgroundWorkers) {
	r := gin.Default()
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Printf("SetTrustedProxies failed: %v", err)
	}
	r.Use(localDevCORS())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.Static("/static", "./.run/uploads")

	// rate_limit
	loginLimiter := ratelimit.Limit(cache, "account_login", 10, time.Minute, ratelimit.KeyByIP)
	registerLimiter := ratelimit.Limit(cache, "account_register", 5, time.Hour, ratelimit.KeyByIP)

	likeLimiter := ratelimit.Limit(cache, "like_write", 30, time.Minute, ratelimit.KeyByAccount)
	commentLimiter := ratelimit.Limit(cache, "comment_write", 10, time.Minute, ratelimit.KeyByAccount)
	socialLimiter := ratelimit.Limit(cache, "social_write", 20, time.Minute, ratelimit.KeyByAccount)
	recommendLimiter := ratelimit.Limit(cache, "recommend_feed", 60, time.Minute, ratelimit.KeyByAccount)
	moderationReportLimiter := ratelimit.Limit(cache, "moderation_report", 10, time.Minute, ratelimit.KeyByAccount)
	moderationReviewLimiter := ratelimit.Limit(cache, "moderation_review", 30, time.Minute, ratelimit.KeyByAccount)

	// account
	accountRepository := account.NewAccountRepository(db)
	accountService := account.NewAccountService(accountRepository, cache)
	accountHandler := account.NewAccountHandler(accountService)
	accountGroup := r.Group("/account")
	{
		accountGroup.POST("/register", registerLimiter, accountHandler.CreateAccount)
		accountGroup.POST("/login", loginLimiter, accountHandler.Login)
		accountGroup.POST("/changePassword", accountHandler.ChangePassword)
		accountGroup.POST("/findByID", accountHandler.FindByID)
		accountGroup.POST("/findByUsername", accountHandler.FindByUsername)
		accountGroup.POST("/refresh", accountHandler.Refresh)
	}
	protectedAccountGroup := accountGroup.Group("")
	protectedAccountGroup.Use(jwt.JWTAuth(accountRepository, cache))
	{
		protectedAccountGroup.POST("/logout", accountHandler.Logout)
		protectedAccountGroup.POST("/rename", accountHandler.Rename)
		protectedAccountGroup.POST("/uploadAvatar", accountHandler.UploadAvatar)
		protectedAccountGroup.POST("/updateProfile", accountHandler.UpdateProfile)
	}

	// video
	videoRepository := video.NewVideoRepository(db)
	popularityMQ, err := rabbitmq.NewPopularityMQ(rmq)
	if err != nil {
		log.Printf("PopularityMQ init failed (mq disabled): %v", err)
		popularityMQ = nil
	}
	videoService := video.NewVideoService(videoRepository, cache, popularityMQ)
	videoHandler := video.NewVideoHandler(videoService)
	chunkHandler := video.NewChunkHandler(cache)
	videoGroup := r.Group("/video")
	{
		videoGroup.POST("/listByAuthorID", videoHandler.ListByAuthorID)
		videoGroup.POST("/getDetail", videoHandler.GetDetail)
		videoGroup.POST("/listDetails", videoHandler.ListDetails)
	}
	protectedVideoGroup := videoGroup.Group("")
	protectedVideoGroup.Use(jwt.JWTAuth(accountRepository, cache))
	{
		protectedVideoGroup.POST("/uploadVideo", videoHandler.UploadVideo)
		protectedVideoGroup.POST("/uploadCover", videoHandler.UploadCover)
		protectedVideoGroup.POST("/publish", videoHandler.PublishVideo)
		protectedVideoGroup.POST("/delete", videoHandler.DeleteVideo)
		protectedVideoGroup.POST("/chunk/init", chunkHandler.InitChunkUpload)
		protectedVideoGroup.POST("/chunk/upload", chunkHandler.UploadChunk)
		protectedVideoGroup.POST("/chunk/status", chunkHandler.ChunkStatus)
		protectedVideoGroup.POST("/chunk/complete", chunkHandler.CompleteChunkUpload)
	}

	// like
	likeMQ, err := rabbitmq.NewLikeMQ(rmq)
	if err != nil {
		log.Printf("LikeMQ init failed (mq disabled): %v", err)
		likeMQ = nil
	}
	likeRepository := video.NewLikeRepository(db)
	likeService := video.NewLikeService(likeRepository, videoRepository, cache, likeMQ, popularityMQ)
	likeHandler := video.NewLikeHandler(likeService)
	likeGroup := r.Group("/like")
	protectedLikeGroup := likeGroup.Group("")
	protectedLikeGroup.Use(jwt.JWTAuth(accountRepository, cache))
	{
		protectedLikeGroup.POST("/like", likeLimiter, likeHandler.Like)
		protectedLikeGroup.POST("/unlike", likeLimiter, likeHandler.UnLike)
		protectedLikeGroup.POST("/isLiked", likeHandler.IsLiked)
		protectedLikeGroup.POST("/listMyLikedVideos", likeHandler.ListMyLikedVideos)
	}

	// comment
	commentRepository := video.NewCommentRepository(db)
	commentMQ, err := rabbitmq.NewCommentMQ(rmq)
	if err != nil {
		log.Printf("CommentMQ init failed (mq disabled): %v", err)
		commentMQ = nil
	}
	commentService := video.NewCommentService(commentRepository, videoRepository, cache, commentMQ, popularityMQ)
	commentHandler := video.NewCommentHandler(commentService, accountService)
	commentGroup := r.Group("/comment")
	{
		commentGroup.POST("/listAll", commentHandler.GetAllComments)
	}
	protectedCommentGroup := commentGroup.Group("")
	protectedCommentGroup.Use(jwt.JWTAuth(accountRepository, cache))
	{
		protectedCommentGroup.POST("/publish", commentLimiter, commentHandler.PublishComment)
		protectedCommentGroup.POST("/delete", commentLimiter, commentHandler.DeleteComment)
	}

	// social
	socialMQ, err := rabbitmq.NewSocialMQ(rmq)
	if err != nil {
		log.Printf("SocialMQ init failed (mq disabled): %v", err)
		socialMQ = nil
	}
	socialRepository := social.NewSocialRepository(db)
	socialService := social.NewSocialService(socialRepository, accountRepository, socialMQ, cache)
	socialHandler := social.NewSocialHandler(socialService)
	socialGroup := r.Group("/social")
	{
		// 粉丝/关注列表对外公开（前端通过传入 id 查询任意用户的列表）。
		socialGroup.POST("/getAllFollowers", socialHandler.GetAllFollowers)
		socialGroup.POST("/getAllVloggers", socialHandler.GetAllVloggers)
	}
	protectedSocialGroup := socialGroup.Group("")
	protectedSocialGroup.Use(jwt.JWTAuth(accountRepository, cache))
	{
		protectedSocialGroup.POST("/follow", socialLimiter, socialHandler.Follow)
		protectedSocialGroup.POST("/unfollow", socialLimiter, socialHandler.Unfollow)
		protectedSocialGroup.POST("/isFollowed", socialHandler.IsFollowed)
		protectedSocialGroup.POST("/getCounts", socialHandler.GetCounts)
	}

	// moderation（提前创建，供推荐流做可见性过滤）
	moderationRepository := moderation.NewRepository(db)
	if err := moderationRepository.AutoMigrate(context.Background()); err != nil {
		log.Printf("moderation AutoMigrate failed: %v", err)
	}
	cfg, _, cfgErr := config.LoadConfig(config.ResolveConfigPath())
	adminIDs := cfg.Moderation.AdminAccountIDs
	if cfgErr != nil {
		log.Printf("moderation: load config failed (%v), admin whitelist empty until MODERATION_ADMIN_IDS is set", cfgErr)
		adminIDs = nil
	}
	adminChecker := moderation.NewStaticAdminChecker(adminIDs)
	if !adminChecker.HasAny() {
		log.Printf("moderation: no admin_account_ids configured; POST /moderation/review will return 403 for everyone")
	}
	moderationService := moderation.NewModerationService(moderationRepository, adminChecker)
	moderationHandler := moderation.NewModerationHandler(moderationService)

	// feed + 推荐（SoftJWT：游客可看列表与推荐，登录后才有 account_id 与曝光去重）
	feedRepository := feed.NewFeedRepository(db)
	feedService := feed.NewFeedService(feedRepository, likeRepository, cache)
	feedHandler := feed.NewFeedHandler(feedService)

	recommendRepository := recommend.NewRepository(db)
	if err := recommendRepository.AutoMigrate(context.Background()); err != nil {
		log.Printf("recommend AutoMigrate failed: %v", err)
	}
	recommendSources := recommend.NewFeedSources(feedService)
	recommendService := recommend.NewRecommendService(
		recommendSources,
		recommend.NewScoreRanker(),
		recommendRepository,
	)
	recommendService.SetVisibility(moderation.NewVideoVisibilityChecker(moderationService))

	feedGroup := r.Group("/feed")
	feedGroup.Use(jwt.SoftJWTAuth(accountRepository, cache))
	{
		feedGroup.POST("/listLatest", feedHandler.ListLatest)
		feedGroup.POST("/listLikesCount", feedHandler.ListByLikes)
		feedGroup.POST("/listByPopularity", feedHandler.ListByPopularity)
		feedGroup.POST("/listByTag", feedHandler.ListByTag)
		feedGroup.POST("/recommend", recommendLimiter, feedRecommend(recommendService))
	}
	protectedFeedGroup := feedGroup.Group("")
	protectedFeedGroup.Use(jwt.JWTAuth(accountRepository, cache))
	{
		protectedFeedGroup.POST("/listByFollowing", feedHandler.ListByFollowing)
	}

	// message
	messageRepository := message.NewRepository(db)
	if err := messageRepository.AutoMigrate(context.Background()); err != nil {
		log.Printf("message AutoMigrate failed: %v", err)
	}
	messageService := message.NewService(messageRepository, accountRepository)
	messageHandler := message.NewHandler(messageService)
	messageGroup := r.Group("/message")
	protectedMessageGroup := messageGroup.Group("")
	protectedMessageGroup.Use(jwt.JWTAuth(accountRepository, cache))
	{
		protectedMessageGroup.POST("/send", messageHandler.Send)
		protectedMessageGroup.POST("/list", messageHandler.List)
		protectedMessageGroup.POST("/conversations", messageHandler.ListConversations)
	}

	// event
	eventRepository := event.NewEventRepository(db)
	if err := eventRepository.AutoMigrate(context.Background()); err != nil {
		log.Printf("event AutoMigrate failed: %v", err)
	}
	eventMQ, err := rabbitmq.NewEventMQ(rmq)
	if err != nil {
		log.Printf("EventMQ init failed (mq disabled): %v", err)
		eventMQ = nil
	}
	eventService := event.NewEventService(eventRepository, videoRepository, eventMQ)
	eventHandler := event.NewEventHandler(eventService)
	eventGroup := r.Group("/event")
	protectedEventGroup := eventGroup.Group("")
	protectedEventGroup.Use(jwt.JWTAuth(accountRepository, cache))
	{
		protectedEventGroup.POST("/track", eventHandler.Track)
		protectedEventGroup.POST("/getVideoMetrics", eventHandler.GetVideoMetrics)
	}

	// moderation（service 已在 feed 段前创建）
	moderationGroup := r.Group("/moderation")
	protectedModerationGroup := moderationGroup.Group("")
	protectedModerationGroup.Use(jwt.JWTAuth(accountRepository, cache))
	{
		protectedModerationGroup.POST("/report", moderationReportLimiter, moderationHandler.Report)
		protectedModerationGroup.POST("/isAdmin", moderationHandler.IsAdmin)
		protectedModerationGroup.POST(
			"/review",
			moderationReviewLimiter,
			moderation.RequireAdmin(adminChecker),
			moderationHandler.Review,
		)
		protectedModerationGroup.POST(
			"/listReports",
			moderation.RequireAdmin(adminChecker),
			moderationHandler.ListReports,
		)
	}

	accountGroup.POST("/getProfile", func(c *gin.Context) {
		var req account.GetProfileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.AccountID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "account_id is required"})
			return
		}
		acc, err := accountService.FindByID(c.Request.Context(), req.AccountID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		videoCount, _ := videoRepository.CountByAuthor(c.Request.Context(), req.AccountID)
		totalLikes, _ := videoRepository.TotalLikesByAuthor(c.Request.Context(), req.AccountID)
		followerCount, _ := socialRepository.CountFollowers(c.Request.Context(), req.AccountID)
		vloggerCount, _ := socialRepository.CountFollowing(c.Request.Context(), req.AccountID)

		c.JSON(http.StatusOK, account.GetProfileResponse{
			Account: account.FindByIDResponse{
				ID: acc.ID, Username: acc.Username, AvatarURL: acc.AvatarURL, Bio: acc.Bio,
			},
			VideoCount: videoCount, TotalLikes: totalLikes,
			FollowerCount: followerCount, VloggerCount: vloggerCount,
		})
	})

	timelineMQ, err := rabbitmq.NewTimelineMQ(rmq)
	if err != nil {
		log.Printf("timelineMQ init failed (mq disabled): %v", err)
		timelineMQ = nil
	}

	if rmq != nil {
		if notifCh, err := rmq.NewChannel(); err == nil {
			if err := rabbitmq.DeclareTopic(notifCh, "like.events", "notification.like", "like.like"); err != nil {
				log.Printf("notification like topic init failed: %v", err)
			}
			if err := rabbitmq.DeclareTopic(notifCh, "comment.events", "notification.comment", "comment.publish"); err != nil {
				log.Printf("notification comment topic init failed: %v", err)
			}
			if err := rabbitmq.DeclareTopic(notifCh, "social.events", "notification.social", "social.follow"); err != nil {
				log.Printf("notification social topic init failed: %v", err)
			}
			_ = notifCh.Close()
		}
	}
	sseHub := worker.NewSSEHub(db)
	notifGroup := r.Group("/notification")
	notifGroup.Use(sseHub.SSERequireAuth())
	sseHub.RegisterRoutes(r, notifGroup)

	bg := newBackgroundWorkers(db, cache, rmq, timelineMQ, eventMQ, sseHub)
	return r, bg
}

func localDevCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if isLocalDevOrigin(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func isLocalDevOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch parsed.Scheme {
	case "http", "https":
	default:
		return false
	}
	switch parsed.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

// feedRecommend 在 SoftJWT 下调用推荐：未登录 accountID=0，仍可混排；登录后才有曝光去重等能力。
func feedRecommend(recommendService *recommend.RecommendService) gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID, _ := jwt.GetAccountID(c)

		var req recommend.RecommendRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		req.Limit = recommend.NormalizeLimit(req.Limit)

		resp, err := recommendService.Recommend(c.Request.Context(), accountID, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if resp.Videos == nil {
			resp.Videos = []recommend.RankedVideo{}
		}
		c.JSON(http.StatusOK, resp)
	}
}
