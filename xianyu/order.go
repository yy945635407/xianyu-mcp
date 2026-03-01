package xianyu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

const boughtURL = "https://www.goofish.com/bought"

type OrderAction struct {
	page *rod.Page
}

type OrderSummary struct {
	SellerName string   `json:"seller_name,omitempty"`
	Status     string   `json:"status,omitempty"`
	Title      string   `json:"title,omitempty"`
	Price      string   `json:"price,omitempty"`
	ItemID     string   `json:"item_id,omitempty"`
	PeerUserID string   `json:"peer_user_id,omitempty"`
	IMLink     string   `json:"im_link,omitempty"`
	Actions    []string `json:"actions,omitempty"`
	RawText    string   `json:"raw_text,omitempty"`
}

type OrderActionResult struct {
	Success      bool   `json:"success"`
	RequiresApp  bool   `json:"requires_app,omitempty"`
	Message      string `json:"message"`
	MatchedOrder string `json:"matched_order,omitempty"`
}

func NewOrderAction(page *rod.Page) *OrderAction {
	return &OrderAction{
		page: page.Timeout(60 * time.Second),
	}
}

func (a *OrderAction) ListOrders(ctx context.Context, tab string, limit int) ([]OrderSummary, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(boughtURL).MustWaitLoad()
	time.Sleep(1200 * time.Millisecond)

	if strings.TrimSpace(tab) != "" && tab != "全部" {
		_ = switchOrderTab(pp, tab)
		time.Sleep(1200 * time.Millisecond)
	}

	if !waitOrderListReady(pp) {
		return nil, fmt.Errorf("order list not ready")
	}

	extract := func() ([]OrderSummary, error) {
		raw := pp.MustEval(`(limit) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const parseIM = (href) => {
			try {
				const u = new URL(href, location.origin);
				return {
					item_id: u.searchParams.get('itemId') || '',
					peer_user_id: u.searchParams.get('peerUserId') || '',
				};
			} catch {
				return { item_id: '', peer_user_id: '' };
			}
		};

		const cards = Array.from(document.querySelectorAll('div[class*="container--Bhfvcld8"]'));
		const out = [];
		for (const card of cards) {
			const seller = clean(card.querySelector('div[class*="name--"] span')?.textContent || '');
			const status = clean(card.querySelector('div[class*="status--"] span')?.textContent || '');
			const title = clean(card.querySelector('div[class*="desc--"]')?.textContent || '');
			const price = clean(card.innerText.match(/¥\s*[0-9]+(?:\.[0-9]+)?/)?.[0] || '');

			const imLinkEl = card.querySelector('a[href*="/im?itemId="]');
			const imLink = imLinkEl ? (imLinkEl.getAttribute('href') || '') : '';
			const parsed = parseIM(imLink);

			const actions = Array.from(card.querySelectorAll('button,div,a,span'))
				.map((el) => clean(el.textContent))
				.filter((t) => t && t.length <= 20)
				.filter((t) => ['联系卖家', '联系买家', '提醒发货', '去发货', '确认收货', '查看物流', '再次购买', '去评价', '查看评价', '退款详情', '更多'].some((k) => t.includes(k)));

			const rawText = clean(card.innerText).slice(0, 220);
			out.push({
				seller_name: seller,
				status,
				title,
				price,
				item_id: parsed.item_id,
				peer_user_id: parsed.peer_user_id,
				im_link: imLink,
				actions,
				raw_text: rawText,
			});
			if (out.length >= limit) break;
		}
		return JSON.stringify(out);
	}`, limit).String()

		var orders []OrderSummary
		if err := json.Unmarshal([]byte(raw), &orders); err != nil {
			return nil, fmt.Errorf("unmarshal orders failed: %w", err)
		}
		return orders, nil
	}

	var orders []OrderSummary
	var err error
	for i := 0; i < 3; i++ {
		orders, err = extract()
		if err != nil {
			return nil, err
		}
		if len(orders) > 0 {
			return orders, nil
		}
		time.Sleep(1200 * time.Millisecond)
	}
	return orders, nil
}

func (a *OrderAction) RemindShip(ctx context.Context, orderKeyword, sellerName string) (*OrderActionResult, error) {
	pp := a.page.Context(ctx)
	pp.MustNavigate(boughtURL).MustWaitLoad()
	time.Sleep(2 * time.Second)

	_ = switchOrderTab(pp, "待发货")
	time.Sleep(1200 * time.Millisecond)

	result := pp.MustEval(`(orderKeyword, sellerName) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const cards = Array.from(document.querySelectorAll('div[class*="container--Bhfvcld8"]'));
		const kw = clean(orderKeyword);
		const seller = clean(sellerName);

		let target = null;
		for (const card of cards) {
			const txt = clean(card.innerText);
			const matchKW = !kw || txt.includes(kw);
			const matchSeller = !seller || txt.includes(seller);
			if (matchKW && matchSeller) {
				target = card;
				break;
			}
		}
		if (!target && cards.length > 0) target = cards[0];
		if (!target) {
			return JSON.stringify({ ok: false, message: '未找到待发货订单', matched: '' });
		}

		const matched = clean(target.querySelector('div[class*="desc--"]')?.textContent || target.innerText).slice(0, 80);
		const controls = Array.from(target.querySelectorAll('button,div,a,span'));
		const remindBtn = controls.find((el) => clean(el.textContent).includes('提醒发货'));
		if (!remindBtn) {
			return JSON.stringify({ ok: false, message: '当前订单无“提醒发货”按钮', matched });
		}

		remindBtn.click();
		return JSON.stringify({ ok: true, message: '已触发提醒发货', matched });
	}`, orderKeyword, sellerName).String()

	type remindResp struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
		Matched string `json:"matched"`
	}
	var resp remindResp
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		return nil, fmt.Errorf("parse remind result failed: %w", err)
	}

	return &OrderActionResult{
		Success:      resp.OK,
		Message:      resp.Message,
		MatchedOrder: resp.Matched,
	}, nil
}

func (a *OrderAction) ShipOrder(ctx context.Context, username string) (*OrderActionResult, error) {
	user := strings.TrimSpace(username)
	if user == "" {
		return nil, fmt.Errorf("username is required for ship_order")
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(imURL).MustWaitLoad()
	time.Sleep(2 * time.Second)
	if !waitIMConversationListReady(pp) {
		return nil, fmt.Errorf("im conversation list not ready")
	}

	clickedName := findConversationByUsername(pp, user)
	if clickedName == "" {
		return nil, fmt.Errorf("conversation not found for user: %s", user)
	}
	if !waitIMConversationReady(pp, clickedName) {
		return nil, fmt.Errorf("conversation opened but UI not ready for user: %s", clickedName)
	}

	clicked := pp.MustEval(`() => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const controls = Array.from(document.querySelectorAll('button,div,span,a'));
		const btn = controls.find((el) => clean(el.textContent) === '去发货' || clean(el.textContent).includes('去发货'));
		if (!btn) return false;
		btn.click();
		return true;
	}`).Bool()
	if !clicked {
		return &OrderActionResult{
			Success:      false,
			Message:      "当前会话未找到“去发货”按钮",
			MatchedOrder: clickedName,
		}, nil
	}
	time.Sleep(1200 * time.Millisecond)

	needApp := pp.MustEval(`() => {
		const txt = (document.body?.innerText || '').replace(/\s+/g, ' ');
		return txt.includes('功能待上线') || txt.includes('扫码去APP查看') || txt.includes('去APP查看');
	}`).Bool()
	if needApp {
		return &OrderActionResult{
			Success:      false,
			RequiresApp:  true,
			Message:      "网页端发货能力受限，请按弹窗提示到闲鱼 APP 完成发货",
			MatchedOrder: clickedName,
		}, nil
	}

	return &OrderActionResult{
		Success:      true,
		Message:      "已触发去发货动作，请在页面确认发货结果",
		MatchedOrder: clickedName,
	}, nil
}

func switchOrderTab(pp *rod.Page, tab string) bool {
	target := strings.TrimSpace(tab)
	if target == "" {
		return false
	}
	return pp.MustEval(`(tab) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const candidates = Array.from(document.querySelectorAll('div,span,a,button'));
		for (const el of candidates) {
			const t = clean(el.textContent);
			if (!t) continue;
			if (t === tab || t.includes(tab)) {
				el.click();
				return true;
			}
		}
		return false;
	}`, target).Bool()
}

func waitOrderListReady(pp *rod.Page) bool {
	for i := 0; i < 20; i++ {
		ready := pp.MustEval(`() => {
			const count = document.querySelectorAll('div[class*="container--Bhfvcld8"]').length;
			if (count > 0) return true;
			const txt = (document.body?.innerText || '').replace(/\s+/g, ' ');
			return txt.includes('暂无相关订单') || txt.includes('暂无订单');
		}`).Bool()
		if ready {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
