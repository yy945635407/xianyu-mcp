package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/sirupsen/logrus"
	"github.com/xpzouying/headless_browser"
	"github.com/ylyt_bot/xianyu-mcp/browser"
	"github.com/ylyt_bot/xianyu-mcp/configs"
	"github.com/ylyt_bot/xianyu-mcp/cookies"
	"github.com/ylyt_bot/xianyu-mcp/xianyu"
)

type XianyuService struct{}

func NewXianyuService() *XianyuService {
	return &XianyuService{}
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
	b := newBrowser()
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xianyu.NewMessageAction(page)
	detail, err := action.SendMessageToUser(ctx, username, message, limit)
	if err != nil {
		return nil, err
	}

	return &SendMessageResponse{
		Username:     username,
		Message:      message,
		Sent:         true,
		Conversation: *detail,
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

func newBrowser() *headless_browser.Browser {
	return browser.NewBrowser(configs.IsHeadless(), browser.WithBinPath(configs.GetBinPath()))
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
