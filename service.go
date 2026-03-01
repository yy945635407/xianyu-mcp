package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/ylyt_bot/xianyu-mcp/browser"
	"github.com/ylyt_bot/xianyu-mcp/configs"
	"github.com/ylyt_bot/xianyu-mcp/cookies"
	"github.com/ylyt_bot/xianyu-mcp/xianyu"
)

type XianyuService struct {
	pullMu         sync.Mutex
	lastIMScanAt   time.Time
	minIMScanEvery time.Duration

	autoCtxMu              sync.Mutex
	autoCtxByUser          map[string]autoContextSnapshot
	autoCtxRefreshInterval time.Duration
	autoCtxCacheTTL        time.Duration
}

type autoContextSnapshot struct {
	ItemRef     string
	OrderStatus string
	UpdatedAt   time.Time
	LastAttempt time.Time
}

func NewXianyuService() *XianyuService {
	minInterval := 8 * time.Second
	if raw := strings.TrimSpace(os.Getenv("XIANYU_IM_SCAN_INTERVAL_MS")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 1000 {
			minInterval = time.Duration(v) * time.Millisecond
		}
	}

	autoCtxRefresh := 3 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("XIANYU_AUTOCTX_REFRESH_SEC")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 30 {
			autoCtxRefresh = time.Duration(v) * time.Second
		}
	}
	autoCtxTTL := 15 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("XIANYU_AUTOCTX_TTL_SEC")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 60 {
			autoCtxTTL = time.Duration(v) * time.Second
		}
	}

	return &XianyuService{
		minIMScanEvery:         minInterval,
		autoCtxByUser:          map[string]autoContextSnapshot{},
		autoCtxRefreshInterval: autoCtxRefresh,
		autoCtxCacheTTL:        autoCtxTTL,
	}
}

func (s *XianyuService) Shutdown() {
	closeSharedBrowser()
}

func (s *XianyuService) shouldScanIMNow(now time.Time) bool {
	s.pullMu.Lock()
	defer s.pullMu.Unlock()

	if s.lastIMScanAt.IsZero() || now.Sub(s.lastIMScanAt) >= s.minIMScanEvery {
		s.lastIMScanAt = now
		return true
	}
	return false
}

func (s *XianyuService) getAutoContextSnapshot(username string, now time.Time) (itemRef, orderStatus string, ok bool) {
	s.autoCtxMu.Lock()
	defer s.autoCtxMu.Unlock()

	key := strings.TrimSpace(username)
	if key == "" {
		return "", "", false
	}
	snap, exists := s.autoCtxByUser[key]
	if !exists || snap.UpdatedAt.IsZero() || now.Sub(snap.UpdatedAt) > s.autoCtxCacheTTL {
		return "", "", false
	}
	return snap.ItemRef, snap.OrderStatus, true
}

func (s *XianyuService) shouldRefreshAutoContext(username string, now time.Time) bool {
	s.autoCtxMu.Lock()
	defer s.autoCtxMu.Unlock()

	key := strings.TrimSpace(username)
	if key == "" {
		return false
	}
	snap, exists := s.autoCtxByUser[key]
	if !exists || snap.LastAttempt.IsZero() || now.Sub(snap.LastAttempt) >= s.autoCtxRefreshInterval {
		snap.LastAttempt = now
		s.autoCtxByUser[key] = snap
		return true
	}
	return false
}

func (s *XianyuService) updateAutoContextSnapshot(username, itemRef, orderStatus string, now time.Time) {
	key := strings.TrimSpace(username)
	if key == "" {
		return
	}
	s.autoCtxMu.Lock()
	defer s.autoCtxMu.Unlock()

	snap := s.autoCtxByUser[key]
	snap.ItemRef = strings.TrimSpace(itemRef)
	snap.OrderStatus = normalizeOrderStatus(orderStatus)
	snap.UpdatedAt = now
	snap.LastAttempt = now
	s.autoCtxByUser[key] = snap
}

func (s *XianyuService) DeleteCookies(ctx context.Context) error {
	cookiePath := cookies.GetCookiesFilePath()
	cookieLoader := cookies.NewLoadCookie(cookiePath)
	return cookieLoader.DeleteCookies()
}

func (s *XianyuService) CheckLoginStatus(ctx context.Context) (*LoginStatusResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	loginAction := xianyu.NewLogin(page)
	isLoggedIn, nickname, err := loginAction.CheckLoginStatus(ctx)
	if err != nil {
		return nil, err
	}

	username := configs.Username
	if nickname != "" {
		username = nickname
	}

	return &LoginStatusResponse{
		IsLoggedIn: isLoggedIn,
		Username:   username,
	}, nil
}

func (s *XianyuService) GetLoginQrcode(ctx context.Context) (*LoginQrcodeResponse, error) {
	b := newBrowser()
	page := b.NewPage()

	deferFunc := func() {
		_ = page.Close()
		b.Close()
	}

	loginAction := xianyu.NewLogin(page)
	img, loggedIn, err := loginAction.FetchQrcodeImage(ctx)
	if err != nil || loggedIn {
		deferFunc()
	}
	if err != nil {
		return nil, err
	}

	timeout := 5 * time.Minute
	if !loggedIn {
		go func() {
			ctxTimeout, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			defer deferFunc()

			if loginAction.WaitForLogin(ctxTimeout) {
				if er := saveCookies(page); er != nil {
					logrus.Errorf("failed to save cookies: %v", er)
				}
			}
		}()
	}

	return &LoginQrcodeResponse{
		Timeout: func() string {
			if loggedIn {
				return "0s"
			}
			return timeout.String()
		}(),
		Img:        img,
		IsLoggedIn: loggedIn,
	}, nil
}

func (s *XianyuService) SearchItems(ctx context.Context, keyword string, limit int) (*SearchItemsResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewSearchAction(page)
	items, err := action.Search(ctx, keyword, limit)
	if err != nil {
		return nil, err
	}

	respItems := make([]SearchItemResponse, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, SearchItemResponse{
			ID:        item.ID,
			Title:     item.Title,
			Price:     item.Price,
			WantCount: item.WantCount,
			URL:       item.URL,
			Seller:    item.Seller,
		})
	}

	return &SearchItemsResponse{
		Keyword: keyword,
		Count:   len(respItems),
		Items:   respItems,
	}, nil
}

func (s *XianyuService) ListConversations(ctx context.Context, limit int) (*ListConversationsResponse, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewMessageAction(page)
	conversations, err := action.ListConversations(ctx, limit)
	if err != nil {
		return nil, err
	}

	return &ListConversationsResponse{
		Count:         len(conversations),
		Conversations: conversations,
	}, nil
}

func (s *XianyuService) GetConversationMessages(ctx context.Context, username string, limit int) (*GetMessagesResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewMessageAction(page)
	detail, err := action.GetConversationByUsername(ctx, username, limit)
	if err != nil {
		return nil, err
	}

	return &GetMessagesResponse{
		Conversation: *detail,
	}, nil
}

func (s *XianyuService) SendMessage(ctx context.Context, username, message string, limit int) (*SendMessageResponse, error) {
	return s.SendMessageWithRequest(ctx, &SendMessageRequest{
		Username: username,
		Message:  message,
		Limit:    limit,
	})
}

func (s *XianyuService) SendMessageWithRequest(ctx context.Context, req *SendMessageRequest) (*SendMessageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("send message request is required")
	}
	username := req.Username
	message := req.Message
	limit := req.Limit
	clientMsgID := req.ClientMsgID

	if !req.Force {
		if sessionStore, err := getIMSessionStore(); err == nil {
			allowed, reason := sessionStore.CheckSendPermission(username, time.Now().UnixMilli())
			if !allowed {
				return &SendMessageResponse{
					Username:    username,
					Message:     message,
					Sent:        false,
					ClientMsgID: clientMsgID,
					Blocked:     true,
					BlockReason: reason,
				}, nil
			}
		} else {
			logrus.Warnf("load session store failed when checking send permission: %v", err)
		}
	}

	recordStore, recErr := getIMSendRecordStore()
	if recErr != nil {
		logrus.Warnf("load send record store failed: %v", recErr)
		recordStore = nil
	}

	if recordStore != nil && strings.TrimSpace(clientMsgID) != "" {
		if rec, ok := recordStore.Get(clientMsgID); ok {
			if rec.Username == strings.TrimSpace(username) && rec.Message == message && rec.Sent {
				resp := rec.Response
				resp.Deduplicated = true
				resp.ClientMsgID = clientMsgID
				if resp.Attempts == 0 {
					resp.Attempts = rec.Attempts
				}
				return &resp, nil
			}
			if rec.Username != strings.TrimSpace(username) || rec.Message != message {
				return nil, fmt.Errorf("client_msg_id already used with different payload")
			}
		}
	}

	maxAttempts := req.MaxRetries
	if maxAttempts <= 0 {
		maxAttempts = 2
	}
	if maxAttempts > 5 {
		maxAttempts = 5
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		b := newBrowser()
		page := b.NewPage()

		action := xianyu.NewMessageAction(page)
		detail, err := action.SendMessageToUser(ctx, username, message, limit)
		_ = page.Close()
		b.Close()
		if err == nil {
			resp := &SendMessageResponse{
				Username:     username,
				Message:      message,
				Sent:         true,
				ClientMsgID:  clientMsgID,
				Attempts:     attempt,
				Conversation: *detail,
			}
			if recordStore != nil {
				if e := recordStore.SaveSuccess(clientMsgID, username, message, attempt, resp); e != nil {
					logrus.Warnf("save send success record failed: %v", e)
				}
			}
			return resp, nil
		}

		lastErr = err
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * 700 * time.Millisecond)
		}
	}

	if recordStore != nil {
		if e := recordStore.SaveFailure(clientMsgID, username, message, maxAttempts, fmt.Sprintf("%v", lastErr)); e != nil {
			logrus.Warnf("save send failure record failed: %v", e)
		}
	}

	return nil, lastErr
}

func (s *XianyuService) PullIMEvents(ctx context.Context, req *PullIMEventsRequest) (*PullIMEventsResponse, error) {
	limit := req.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	scanLimit := req.ScanLimit
	if scanLimit <= 0 || scanLimit > 200 {
		scanLimit = 30
	}

	store, err := getIMEventStore()
	if err != nil {
		return nil, err
	}

	generatedCount := 0
	if s.shouldScanIMNow(time.Now()) {
		conversations, listErr := s.ListConversations(ctx, scanLimit)
		if listErr != nil {
			return nil, listErr
		}
		generated, capErr := store.CaptureConversations(conversations.Conversations)
		if capErr != nil {
			return nil, capErr
		}
		generatedCount = len(generated)
	}

	events, nextCursor := store.ListSinceID(req.SinceID, limit)
	return &PullIMEventsResponse{
		SinceID:    req.SinceID,
		NextCursor: nextCursor,
		Generated:  generatedCount,
		Count:      len(events),
		Events:     events,
	}, nil
}

func (s *XianyuService) GetIMSessionState(ctx context.Context, username string) (*IMSessionStateResponse, error) {
	store, err := getIMSessionStore()
	if err != nil {
		return nil, err
	}
	state := store.Get(username)
	return &IMSessionStateResponse{State: state}, nil
}

func (s *XianyuService) ListIMSessionStates(ctx context.Context, limit int) (*IMSessionStatesResponse, error) {
	store, err := getIMSessionStore()
	if err != nil {
		return nil, err
	}
	states := store.List(limit)
	return &IMSessionStatesResponse{
		Count:  len(states),
		States: states,
	}, nil
}

func (s *XianyuService) SetIMSessionState(ctx context.Context, req *SetIMSessionStateRequest) (*IMSessionStateResponse, error) {
	store, err := getIMSessionStore()
	if err != nil {
		return nil, err
	}
	state, err := store.Upsert(req.Username, req.Mode, req.HandoffReason, req.LockOwner, req.LockSeconds, req.ClearLock)
	if err != nil {
		return nil, err
	}
	return &IMSessionStateResponse{State: state}, nil
}

func (s *XianyuService) MarkIMSessionRead(ctx context.Context, req *MarkIMSessionReadRequest) (*IMSessionStateResponse, error) {
	limit := req.Limit
	if limit <= 0 || limit > 500 {
		limit = 30
	}

	b := newBrowser()
	defer b.Close()
	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewMessageAction(page)
	_, err := action.GetConversationByUsername(ctx, req.Username, limit)
	if err != nil {
		logrus.Warnf("mark read by opening conversation failed (fallback to local state): %v", err)
	}

	store, err := getIMSessionStore()
	if err != nil {
		return nil, err
	}
	state, err := store.MarkRead(req.Username, time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	return &IMSessionStateResponse{State: state}, nil
}

func normalizeMatchText(v string) string {
	replacer := strings.NewReplacer(
		"\n", " ",
		"\t", " ",
		",", " ",
		"，", " ",
		".", " ",
		"。", " ",
		":", " ",
		"：", " ",
		";", " ",
		"；", " ",
		"?", " ",
		"？", " ",
		"!", " ",
		"！", " ",
		"(", " ",
		")", " ",
		"（", " ",
		"）", " ",
	)
	cleaned := strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
	return strings.Join(strings.Fields(cleaned), " ")
}

func compactMatchText(v string) string {
	return strings.ReplaceAll(v, " ", "")
}

func containsFuzzy(a, b string) bool {
	left := compactMatchText(normalizeMatchText(a))
	right := compactMatchText(normalizeMatchText(b))
	if left == "" || right == "" {
		return false
	}
	return strings.Contains(left, right) || strings.Contains(right, left)
}

func containsStatus(statuses []string, target string) bool {
	for _, st := range statuses {
		if st == target {
			return true
		}
	}
	return false
}

func (s *XianyuService) UpsertIMKnowledge(ctx context.Context, req *UpsertIMKnowledgeRequest) (*IMKnowledgeEntryResponse, error) {
	store, err := getIMKnowledgeStore()
	if err != nil {
		return nil, err
	}
	entry, err := store.Upsert(req)
	if err != nil {
		return nil, err
	}
	return &IMKnowledgeEntryResponse{Entry: entry}, nil
}

func (s *XianyuService) ListIMKnowledge(ctx context.Context, req *ListIMKnowledgeRequest) (*IMKnowledgeListResponse, error) {
	store, err := getIMKnowledgeStore()
	if err != nil {
		return nil, err
	}

	limit := req.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	itemRef := strings.TrimSpace(req.ItemRef)
	status := normalizeOrderStatus(req.OrderStatus)
	queryNorm := normalizeMatchText(req.Query)
	enabled := req.Enabled

	allEntries := store.ListAll()
	filtered := make([]IMKnowledgeEntry, 0, len(allEntries))
	for _, entry := range allEntries {
		if enabled != nil && entry.Enabled != *enabled {
			continue
		}
		if itemRef != "" && entry.ItemRef != "" && !containsFuzzy(itemRef, entry.ItemRef) {
			continue
		}
		if status != "" && len(entry.OrderStatuses) > 0 && !containsStatus(entry.OrderStatuses, status) {
			continue
		}
		if queryNorm != "" {
			matched := containsFuzzy(entry.Title, queryNorm) || containsFuzzy(entry.Answer, queryNorm)
			if !matched {
				for _, kw := range entry.Keywords {
					if containsFuzzy(kw, queryNorm) {
						matched = true
						break
					}
				}
			}
			if !matched {
				for _, tag := range entry.Tags {
					if containsFuzzy(tag, queryNorm) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, entry)
	}

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return &IMKnowledgeListResponse{
		Count:   len(filtered),
		Entries: filtered,
	}, nil
}

func (s *XianyuService) DeleteIMKnowledge(ctx context.Context, req *DeleteIMKnowledgeRequest) (*DeleteIMKnowledgeResponse, error) {
	store, err := getIMKnowledgeStore()
	if err != nil {
		return nil, err
	}
	deleted, err := store.Delete(req.ID)
	if err != nil {
		return nil, err
	}
	return &DeleteIMKnowledgeResponse{
		ID:      strings.TrimSpace(req.ID),
		Deleted: deleted,
	}, nil
}

func (s *XianyuService) MatchIMKnowledge(ctx context.Context, req *MatchIMKnowledgeRequest) (*MatchIMKnowledgeResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	question := strings.TrimSpace(req.Message)
	if question == "" {
		return nil, fmt.Errorf("message is required")
	}

	itemRef := strings.TrimSpace(req.ItemRef)
	orderStatus := normalizeOrderStatus(req.OrderStatus)
	username := strings.TrimSpace(req.Username)

	if req.AutoContext && username != "" && (itemRef == "" || orderStatus == "") {
		now := time.Now()
		if cachedRef, cachedStatus, ok := s.getAutoContextSnapshot(username, now); ok {
			if itemRef == "" {
				itemRef = cachedRef
			}
			if orderStatus == "" {
				orderStatus = cachedStatus
			}
		}

		if (itemRef == "" || orderStatus == "") && s.shouldRefreshAutoContext(username, now) {
			autoCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			detail, err := s.GetConversationMessages(autoCtx, username, 30)
			cancel()
			if err != nil {
				logrus.Warnf("load conversation for kb auto context failed: %v", err)
			} else {
				resolvedRef := strings.TrimSpace(detail.Conversation.ProductContext.ProductRef)
				resolvedStatus := normalizeOrderStatus(detail.Conversation.OrderStatus)
				s.updateAutoContextSnapshot(username, resolvedRef, resolvedStatus, now)
				if itemRef == "" {
					itemRef = resolvedRef
				}
				if orderStatus == "" {
					orderStatus = resolvedStatus
				}
			}
		}
	}

	topK := req.TopK
	if topK <= 0 || topK > 20 {
		topK = 3
	}

	store, err := getIMKnowledgeStore()
	if err != nil {
		return nil, err
	}

	questionNorm := normalizeMatchText(question)
	questionCompact := compactMatchText(questionNorm)

	type scoredEntry struct {
		Entry   IMKnowledgeEntry
		Score   int
		Matched []string
	}

	scored := make([]scoredEntry, 0, 32)
	for _, entry := range store.ListAll() {
		if !entry.Enabled {
			continue
		}

		score := entry.Priority
		matchedKeywords := make([]string, 0, len(entry.Keywords))
		for _, kw := range entry.Keywords {
			kwNorm := normalizeMatchText(kw)
			if kwNorm == "" {
				continue
			}
			kwCompact := compactMatchText(kwNorm)
			if strings.Contains(questionNorm, kwNorm) || strings.Contains(questionCompact, kwCompact) {
				matchedKeywords = append(matchedKeywords, kw)
				score += 20 + len([]rune(kwNorm))/4
			}
		}

		if len(matchedKeywords) == 0 {
			continue
		}

		if itemRef != "" {
			if entry.ItemRef == "" {
				score += 2
			} else if containsFuzzy(itemRef, entry.ItemRef) {
				score += 12
			} else {
				continue
			}
		}

		if orderStatus != "" {
			if len(entry.OrderStatuses) == 0 {
				score += 1
			} else if containsStatus(entry.OrderStatuses, orderStatus) {
				score += 8
			} else {
				continue
			}
		}

		if entry.Title != "" && containsFuzzy(question, entry.Title) {
			score += 3
		}
		if containsFuzzy(question, entry.Answer) {
			score += 2
		}

		scored = append(scored, scoredEntry{
			Entry:   entry,
			Score:   score,
			Matched: matchedKeywords,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Entry.UpdatedAt > scored[j].Entry.UpdatedAt
	})
	if len(scored) > topK {
		scored = scored[:topK]
	}

	matches := make([]IMKnowledgeMatch, 0, len(scored))
	for _, row := range scored {
		matches = append(matches, IMKnowledgeMatch{
			ID:              row.Entry.ID,
			Title:           row.Entry.Title,
			Answer:          row.Entry.Answer,
			ItemRef:         row.Entry.ItemRef,
			OrderStatuses:   row.Entry.OrderStatuses,
			Tags:            row.Entry.Tags,
			Score:           row.Score,
			MatchedKeywords: row.Matched,
		})
	}

	bestAnswer := ""
	if len(matches) > 0 {
		bestAnswer = matches[0].Answer
	}

	return &MatchIMKnowledgeResponse{
		Message:     question,
		Username:    username,
		ItemRef:     itemRef,
		OrderStatus: orderStatus,
		Count:       len(matches),
		BestAnswer:  bestAnswer,
		Matches:     matches,
	}, nil
}

func (s *XianyuService) PublishItem(ctx context.Context, req *PublishItemRequest) (*PublishItemResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewPublishItemAction(page)
	result, err := action.Publish(ctx, xianyu.PublishItemContent{
		Images:            req.Images,
		Description:       req.Description,
		Price:             req.Price,
		OriginalPrice:     req.OriginalPrice,
		ShippingType:      req.ShippingType,
		ShippingFee:       req.ShippingFee,
		SupportSelfPickup: req.SupportSelfPickup,
		LocationKeyword:   req.LocationKeyword,
		SpecTypes:         req.SpecTypes,
		Submit:            req.Submit,
	})
	if err != nil {
		return nil, err
	}

	return &PublishItemResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) ListOrders(ctx context.Context, tab string, limit int) (*ListOrdersResponse, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewOrderAction(page)
	orders, err := action.ListOrders(ctx, tab, limit)
	if err != nil {
		return nil, err
	}

	return &ListOrdersResponse{
		Count:  len(orders),
		Orders: orders,
	}, nil
}

func (s *XianyuService) RemindShip(ctx context.Context, req *RemindShipRequest) (*OrderActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewOrderAction(page)
	result, err := action.RemindShip(ctx, req.OrderKeyword, req.SellerName)
	if err != nil {
		return nil, err
	}

	return &OrderActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) ShipOrder(ctx context.Context, req *ShipOrderRequest) (*OrderActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewOrderAction(page)
	result, err := action.ShipOrder(ctx, req.Username)
	if err != nil {
		return nil, err
	}

	return &OrderActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) ListCollections(ctx context.Context, group string, limit int) (*ListCollectionsResponse, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewCollectionAction(page)
	result, err := action.ListCollections(ctx, group, limit)
	if err != nil {
		return nil, err
	}

	return &ListCollectionsResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) CancelFavorite(ctx context.Context, req *CancelFavoriteRequest) (*CollectionActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewCollectionAction(page)
	result, err := action.CancelFavorite(ctx, req.Keyword, req.ItemRef)
	if err != nil {
		return nil, err
	}

	return &CollectionActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) ManageCollectionGroup(ctx context.Context, req *ManageCollectionGroupRequest) (*CollectionActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewCollectionAction(page)
	result, err := action.ManageGroup(ctx, req.Operation, req.GroupName, req.NewName, req.ItemKeyword)
	if err != nil {
		return nil, err
	}

	return &CollectionActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) ListMyItems(ctx context.Context, tab string, limit int) (*ListMyItemsResponse, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewMyItemsAction(page)
	result, err := action.ListMyItems(ctx, tab, limit)
	if err != nil {
		return nil, err
	}

	return &ListMyItemsResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) EditMyItem(ctx context.Context, req *EditMyItemRequest) (*MyItemActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewMyItemsAction(page)
	result, err := action.EditMyItem(ctx, xianyu.EditMyItemParams{
		Keyword:     req.Keyword,
		ItemRef:     req.ItemRef,
		Tab:         req.Tab,
		Price:       req.Price,
		Description: req.Description,
		Submit:      req.Submit,
	})
	if err != nil {
		return nil, err
	}

	return &MyItemActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) ShelfMyItem(ctx context.Context, req *ShelfMyItemRequest) (*MyItemActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewMyItemsAction(page)
	result, err := action.ShelfMyItem(ctx, req.Keyword, req.ItemRef, req.Tab, req.Action)
	if err != nil {
		return nil, err
	}

	return &MyItemActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) DeleteMyItem(ctx context.Context, req *DeleteMyItemRequest) (*MyItemActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewMyItemsAction(page)
	result, err := action.DeleteMyItem(ctx, req.Keyword, req.ItemRef, req.Tab)
	if err != nil {
		return nil, err
	}

	return &MyItemActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) GetItemDetail(ctx context.Context, itemRef string) (*GetItemDetailResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewItemDetailAction(page)
	result, err := action.GetDetail(ctx, itemRef)
	if err != nil {
		return nil, err
	}

	return &GetItemDetailResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) FavoriteItem(ctx context.Context, req *FavoriteItemRequest) (*ItemOperateResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewItemDetailAction(page)
	result, err := action.Favorite(ctx, req.ItemRef)
	if err != nil {
		return nil, err
	}

	return &ItemOperateResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) ChatItem(ctx context.Context, req *ChatItemRequest) (*ItemOperateResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewItemDetailAction(page)
	result, err := action.Chat(ctx, req.ItemRef, req.Message)
	if err != nil {
		return nil, err
	}

	return &ItemOperateResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) BuyItem(ctx context.Context, req *BuyItemRequest) (*ItemOperateResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewItemDetailAction(page)
	result, err := action.Buy(ctx, req.ItemRef)
	if err != nil {
		return nil, err
	}

	return &ItemOperateResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) GetAccountSecurity(ctx context.Context) (*AccountSecurityResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewAccountAction(page)
	result, err := action.GetAccountSecurity(ctx)
	if err != nil {
		return nil, err
	}

	return &AccountSecurityResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) GetCommunityFeed(ctx context.Context, keyword string, limit int) (*CommunityFeedResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewCommunityServiceAction(page)
	result, err := action.GetCommunityFeed(ctx, keyword, limit)
	if err != nil {
		return nil, err
	}

	return &CommunityFeedResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) InteractCommunity(ctx context.Context, req *InteractCommunityRequest) (*CommunityActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewCommunityServiceAction(page)
	result, err := action.InteractCommunity(ctx, req.Keyword, req.Action)
	if err != nil {
		return nil, err
	}

	return &CommunityActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) GetCustomerService(ctx context.Context, afterSaleLimit int) (*CustomerServiceResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewCommunityServiceAction(page)
	result, err := action.GetCustomerService(ctx, afterSaleLimit)
	if err != nil {
		return nil, err
	}

	return &CustomerServiceResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) OpenCustomerService(ctx context.Context, req *OpenCustomerServiceRequest) (*CommunityActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewCommunityServiceAction(page)
	result, err := action.OpenCustomerService(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return &CommunityActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) ShipWithLogistics(ctx context.Context, req *ShipWithLogisticsRequest) (*OrderActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewOrderAction(page)
	result, err := action.ShipWithLogistics(ctx, req.Username, req.Company, req.TrackingNo)
	if err != nil {
		return nil, err
	}

	return &OrderActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) ConfirmReceipt(ctx context.Context, req *ConfirmReceiptRequest) (*OrderActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewOrderAction(page)
	result, err := action.ConfirmReceipt(ctx, req.OrderKeyword, req.SellerName)
	if err != nil {
		return nil, err
	}

	return &OrderActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) ReviewOrder(ctx context.Context, req *ReviewOrderRequest) (*OrderActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewOrderAction(page)
	result, err := action.ReviewOrder(ctx, req.OrderKeyword, req.SellerName, req.Score, req.Content)
	if err != nil {
		return nil, err
	}

	return &OrderActionResponse{
		Result: *result,
	}, nil
}

func (s *XianyuService) HandleRefund(ctx context.Context, req *RefundActionRequest) (*OrderActionResponse, error) {
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewOrderAction(page)
	result, err := action.HandleRefund(ctx, req.OrderKeyword, req.SellerName, req.Action, req.Reason)
	if err != nil {
		return nil, err
	}

	return &OrderActionResponse{
		Result: *result,
	}, nil
}

type browserSession struct {
	browser *headless_browser.Browser
	closed  bool
}

func (s *browserSession) NewPage() *rod.Page {
	return s.browser.NewPage()
}

func (s *browserSession) Close() {
	sharedBrowserMu.Lock()
	defer sharedBrowserMu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	if sharedBrowserRefCount > 0 {
		sharedBrowserRefCount--
	}
}

var (
	sharedBrowserMu       sync.Mutex
	sharedBrowserInst     *headless_browser.Browser
	sharedBrowserRefCount int
	sharedBrowserHeadless bool
	sharedBrowserBinPath  string
)

func newBrowser() *browserSession {
	sharedBrowserMu.Lock()
	defer sharedBrowserMu.Unlock()

	headless := configs.IsHeadless()
	binPath := configs.GetBinPath()
	needRecreate := sharedBrowserInst == nil || sharedBrowserHeadless != headless || sharedBrowserBinPath != binPath
	if needRecreate {
		if sharedBrowserInst != nil {
			sharedBrowserInst.Close()
		}
		sharedBrowserInst = browser.NewBrowser(headless, browser.WithBinPath(binPath))
		sharedBrowserHeadless = headless
		sharedBrowserBinPath = binPath
		sharedBrowserRefCount = 0
	}

	sharedBrowserRefCount++
	return &browserSession{browser: sharedBrowserInst}
}

func closeSharedBrowser() {
	sharedBrowserMu.Lock()
	defer sharedBrowserMu.Unlock()

	if sharedBrowserInst != nil {
		sharedBrowserInst.Close()
	}
	sharedBrowserInst = nil
	sharedBrowserRefCount = 0
	sharedBrowserHeadless = false
	sharedBrowserBinPath = ""
}

func saveCookies(page *rod.Page) error {
	cks, err := page.Browser().GetCookies()
	if err != nil {
		return err
	}

	data, err := json.Marshal(cks)
	if err != nil {
		return err
	}

	cookieLoader := cookies.NewLoadCookie(cookies.GetCookiesFilePath())
	if err := cookieLoader.SaveCookies(data); err != nil {
		return err
	}

	logrus.Infof("cookies saved: %s", cookies.GetCookiesFilePath())
	return nil
}

func formatLoginDeadline(now time.Time, timeout string) string {
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return now.Format("2006-01-02 15:04:05")
	}
	return now.Add(d).Format("2006-01-02 15:04:05")
}

func loginInstructionText(deadline string) string {
	return fmt.Sprintf("请在 %s 前使用闲鱼 App 扫码登录", deadline)
}
