package xianyu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

const imURL = "https://www.goofish.com/im"

type MessageAction struct {
	page *rod.Page
}

type ConversationSummary struct {
	Username     string `json:"username"`
	LastMessage  string `json:"last_message"`
	LastTime     string `json:"last_time,omitempty"`
	OrderStatus  string `json:"order_status"`
	StatusTag    string `json:"status_tag,omitempty"`
	UnreadCount  int    `json:"unread_count,omitempty"`
	IsSystem     bool   `json:"is_system,omitempty"`
	RawPreview   string `json:"raw_preview,omitempty"`
	Conversation int    `json:"conversation_index,omitempty"`
}

type ProductContext struct {
	Title       string `json:"title,omitempty"`
	Price       string `json:"price,omitempty"`
	ShippingFee string `json:"shipping_fee,omitempty"`
	Location    string `json:"location,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	ItemID      string `json:"item_id,omitempty"`
	ProductRef  string `json:"product_ref,omitempty"`
}

type ConversationMessage struct {
	Sender    string `json:"sender,omitempty"`
	Direction string `json:"direction"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	Time      string `json:"time,omitempty"`
}

type ConversationDetail struct {
	Username       string                `json:"username"`
	Alias          string                `json:"alias,omitempty"`
	UserID         string                `json:"user_id,omitempty"`
	ProductContext ProductContext        `json:"product_context"`
	OrderStatus    string                `json:"order_status"`
	Messages       []ConversationMessage `json:"messages"`
}

func NewMessageAction(page *rod.Page) *MessageAction {
	return &MessageAction{
		page: page.Timeout(60 * time.Second),
	}
}

func (a *MessageAction) ListConversations(ctx context.Context, limit int) ([]ConversationSummary, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(imURL).MustWaitLoad()
	if !waitIMConversationListReady(pp) {
		return nil, fmt.Errorf("im conversation list not ready")
	}

	extract := func() ([]ConversationSummary, error) {
		raw := pp.MustEval(`(limit) => {
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const isTime = (s) => /(今天|昨天|前天|刚刚|\d{1,2}分钟前|\d{1,2}小时前|\d{2}-\d{2}|\d{2}:\d{2})/.test(s);
			const isStatus = (s) => /交易成功|有新交易评价|待付款|待发货|待收货|已收货/.test(s);

			const normalizeOrderStatus = (txt) => {
				if (!txt) return '未下单';
				if (txt.includes('买家已确认收货') || txt.includes('交易成功')) return '已收货';
				if (txt.includes('你已发货') || txt.includes('我已发货') || txt.includes('发货凭证')) return '我已发货';
				if (txt.includes('我已付款') || txt.includes('等待你发货') || txt.includes('我已拍下') || txt.includes('待付款') || txt.includes('待发货')) return '已拍下';
				return '未下单';
			};

			const result = [];
			const items = Array.from(document.querySelectorAll('div[class*="conversation-item"]'));
			for (let i = 0; i < items.length; i++) {
				const item = items[i];
				const full = clean(item.innerText);
				if (!full) continue;

				let lines = (item.innerText || '').split('\n').map((s) => clean(s)).filter(Boolean);
				if (lines.length === 0) continue;
				if (/^\d+$/.test(lines[0])) lines = lines.slice(1);
				const username = lines[0] || '';
				if (!username || username.includes('通知消息')) {
					continue;
				}

				const statusTag = lines.find((l) => isStatus(l)) || clean(item.querySelector('div[class*="order-success"]')?.textContent || '');
				const unreadText = clean(item.querySelector('.ant-badge-count')?.textContent || '');
				const unreadCount = parseInt(unreadText, 10) || 0;

				let lastTime = '';
				if (lines.length > 0 && isTime(lines[lines.length - 1])) {
					lastTime = lines[lines.length - 1];
				}

				const bodyLines = lines.slice(1).filter((l) => l !== lastTime && l !== statusTag);
				let msg = bodyLines[0] || '';
				if (!msg) {
					msg = full;
				}

				const orderStatus = normalizeOrderStatus(statusTag + ' ' + msg + ' ' + full);

				result.push({
					username,
					last_message: msg,
					last_time: lastTime,
					status_tag: statusTag,
					order_status: orderStatus,
					unread_count: unreadCount,
					raw_preview: full.slice(0, 120),
					conversation_index: i,
				});

				if (result.length >= limit) break;
			}

			return JSON.stringify(result);
		}`, limit).String()

		var conversations []ConversationSummary
		if err := json.Unmarshal([]byte(raw), &conversations); err != nil {
			return nil, fmt.Errorf("unmarshal conversations failed: %w", err)
		}
		return conversations, nil
	}

	var conversations []ConversationSummary
	var err error
	for i := 0; i < 3; i++ {
		conversations, err = extract()
		if err != nil {
			return nil, err
		}
		if len(conversations) > 0 {
			return conversations, nil
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return conversations, nil
}

func (a *MessageAction) GetConversationByUsername(ctx context.Context, username string, limit int) (*ConversationDetail, error) {
	if strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("username is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(imURL).MustWaitLoad()
	if !waitIMConversationListReady(pp) {
		return nil, fmt.Errorf("im conversation list not ready")
	}

	clickedName := findConversationByUsername(pp, username)
	if clickedName == "" {
		time.Sleep(1200 * time.Millisecond)
		clickedName = findConversationByUsername(pp, username)
	}
	if clickedName == "" {
		return nil, fmt.Errorf("conversation not found for user: %s", username)
	}

	if !waitIMConversationReady(pp, clickedName) {
		return nil, fmt.Errorf("conversation opened but UI not ready for user: %s", clickedName)
	}

	raw := pp.MustEval(`(limit) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();

		const normalizeOrderStatus = (txt) => {
			if (!txt) return '未下单';
			if (txt.includes('买家已确认收货') || txt.includes('交易成功')) return '已收货';
			if (txt.includes('你已发货') || txt.includes('我已发货') || txt.includes('发货凭证')) return '我已发货';
			if (txt.includes('我已付款') || txt.includes('等待你发货') || txt.includes('我已拍下') || txt.includes('待付款') || txt.includes('待发货')) return '已拍下';
			return '未下单';
		};

		const detail = {
			username: '',
			alias: '',
			user_id: '',
			product_context: {
				title: '',
				price: '',
				shipping_fee: '',
				location: '',
				image_url: '',
				item_id: '',
				product_ref: '',
			},
			order_status: '未下单',
			messages: [],
		};

		const topbar = document.querySelector('div[class*="message-topbar"]');
		if (topbar) {
			detail.username = clean(topbar.querySelector('span[class*="text1"]')?.textContent || '');
			detail.alias = clean(topbar.querySelector('span[class*="text2"]')?.textContent || '');
			const userLink = topbar.querySelector('a[href*="personal?userId="]');
			const userHref = userLink ? userLink.getAttribute('href') || '' : '';
			const m = userHref.match(/userId=(\d+)/);
			if (m) detail.user_id = m[1];
		}

		const itemIdInURL = new URLSearchParams(location.search).get('itemId') || '';
		if (itemIdInURL) detail.product_context.item_id = itemIdInURL;

		const product = document.querySelector('div[class*="container--dgZTBkgv"]');
		if (product) {
			detail.product_context.image_url = (product.querySelector('img')?.src || '').trim();
			detail.product_context.price = clean(product.querySelector('div[class*="money--"]')?.textContent || '');
			const deliveryNodes = Array.from(product.querySelectorAll('div[class*="delivery--"]')).map((el) => clean(el.textContent)).filter(Boolean);
			if (deliveryNodes.length > 0) detail.product_context.shipping_fee = deliveryNodes[0];
			if (deliveryNodes.length > 1) detail.product_context.location = deliveryNodes[1];
			const titleFromDesc = clean(product.querySelector('div[class*="desc"],div[class*="title"]')?.textContent || '');
			detail.product_context.title = titleFromDesc;
		}

		const rows = Array.from(document.querySelectorAll('li.ant-list-item'));
		const picked = rows.slice(Math.max(0, rows.length - limit));
		for (const li of picked) {
			const text = clean(li.innerText);
			if (!text) continue;

			const right = !!li.querySelector('div[class*="message-text-right"]');
			const left = !!li.querySelector('div[class*="message-text-left"]');
			let direction = 'system';
			if (right) direction = 'out';
			if (left) direction = 'in';

			let msgType = 'text';
			if (li.querySelector('div[class*="msg-dx-title"]')) {
				msgType = 'trade';
			} else if (text.includes('图片')) {
				msgType = 'image';
			}

			let content = '';
			if (msgType === 'trade') {
				const title = clean(li.querySelector('div[class*="msg-dx-title"]')?.textContent || '');
				const desc = clean(li.querySelector('div[class*="msg-dx-desc"]')?.textContent || '');
				const btn = clean(li.querySelector('div[class*="msg-dx-button-text"]')?.textContent || '');
				content = clean([title, desc, btn].filter(Boolean).join(" | "));
			} else {
				const bubble = clean(li.querySelector('div[class*="message-text--"]')?.textContent || '');
				content = bubble || text;
			}
			if (!content) continue;

			const sender = direction === 'in' ? detail.username : (direction === 'out' ? '我' : '系统');
			const time = clean(li.querySelector('div[class*="time"],span[class*="time"]')?.textContent || '');

			detail.messages.push({
				sender,
				direction,
				type: msgType,
				content,
				time,
			});
		}

		const statusTextRaw = clean(document.body?.innerText || '').slice(0, 5000);
		detail.order_status = normalizeOrderStatus(statusTextRaw);

		const refParts = [
			detail.product_context.title,
			detail.product_context.price,
			detail.product_context.location,
			detail.product_context.item_id,
		].filter(Boolean);
		detail.product_context.product_ref = refParts.join(" | ");
		if (!detail.product_context.product_ref && detail.product_context.image_url) {
			detail.product_context.product_ref = detail.product_context.image_url.slice(-32);
		}

		return JSON.stringify(detail);
	}`, limit).String()

	var detail ConversationDetail
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		return nil, fmt.Errorf("unmarshal conversation detail failed: %w", err)
	}

	if detail.Username == "" {
		detail.Username = clickedName
	}
	if detail.OrderStatus == "" {
		detail.OrderStatus = "未下单"
	}

	return &detail, nil
}

func (a *MessageAction) SendMessageToUser(ctx context.Context, username, message string, verifyLimit int) (*ConversationDetail, error) {
	if strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("username is required")
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return nil, fmt.Errorf("message is required")
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(imURL).MustWaitLoad()
	if !waitIMConversationListReady(pp) {
		return nil, fmt.Errorf("im conversation list not ready")
	}

	clickedName := findConversationByUsername(pp, username)
	if clickedName == "" {
		time.Sleep(1200 * time.Millisecond)
		clickedName = findConversationByUsername(pp, username)
	}
	if clickedName == "" {
		return nil, fmt.Errorf("conversation not found for user: %s", username)
	}

	if !waitIMConversationReady(pp, clickedName) {
		return nil, fmt.Errorf("conversation opened but UI not ready for user: %s", clickedName)
	}

	type messageSnapshot struct {
		Count       int    `json:"count"`
		Last        string `json:"last"`
		TailMatches int    `json:"tail_matches"`
	}

	type sendAttemptResult struct {
		InputFound     bool   `json:"input_found"`
		ButtonFound    bool   `json:"button_found"`
		ButtonDisabled bool   `json:"button_disabled"`
		Clicked        bool   `json:"clicked"`
		ButtonText     string `json:"button_text"`
		ButtonClass    string `json:"button_class"`
	}

	readSnapshot := func(targetText string) (*messageSnapshot, error) {
		raw := pp.MustEval(`(targetText) => {
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const rows = Array.from(document.querySelectorAll('li.ant-list-item'));
			const tail = rows.slice(Math.max(0, rows.length - 20)).map((li) => clean(li.innerText || '')).filter(Boolean);
			const target = clean(targetText);
			return JSON.stringify({
				count: rows.length,
				last: tail.length > 0 ? tail[tail.length - 1] : '',
				tail_matches: tail.filter((t) => target && t.includes(target)).length,
			});
		}`, targetText).String()

		var snap messageSnapshot
		if err := json.Unmarshal([]byte(raw), &snap); err != nil {
			return nil, fmt.Errorf("unmarshal message snapshot failed: %w", err)
		}
		return &snap, nil
	}

	beforeSnap, err := readSnapshot(msg)
	if err != nil {
		return nil, err
	}

	var sendResult sendAttemptResult
	sentTriggered := false
	var lastSendErr error

	for i := 0; i < 3; i++ {
		raw := pp.MustEval(`(text) => {
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const norm = (s) => clean(s).replace(/\s+/g, '');

			const input = document.querySelector('textarea[placeholder*="请输入消息"], textarea[class*="textarea-no-border"], textarea.ant-input, textarea');
			if (!input) {
				return JSON.stringify({
					input_found: false,
					button_found: false,
					button_disabled: false,
					clicked: false,
					button_text: '',
					button_class: '',
				});
			}

			input.focus();
			const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value')?.set;
			if (setter) {
				setter.call(input, '');
				input.dispatchEvent(new Event('input', { bubbles: true }));
				setter.call(input, text);
				input.dispatchEvent(new Event('input', { bubbles: true }));
			} else {
				input.value = text;
				input.dispatchEvent(new Event('input', { bubbles: true }));
			}
			input.dispatchEvent(new Event('change', { bubbles: true }));

			const btnCandidates = Array.from(document.querySelectorAll('div[class*="sendbox-bottom"] button, button'));
			const sendBtn = btnCandidates.find((el) => norm(el.textContent || '') === '发送');
			if (!sendBtn) {
				return JSON.stringify({
					input_found: true,
					button_found: false,
					button_disabled: false,
					clicked: false,
					button_text: '',
					button_class: '',
				});
			}

			const disabled = !!sendBtn.disabled || sendBtn.getAttribute('aria-disabled') === 'true';
			if (disabled) {
				return JSON.stringify({
					input_found: true,
					button_found: true,
					button_disabled: true,
					clicked: false,
					button_text: clean(sendBtn.textContent || ''),
					button_class: (sendBtn.className || '').toString(),
				});
			}

			sendBtn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
			sendBtn.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
			sendBtn.click();

			return JSON.stringify({
				input_found: true,
				button_found: true,
				button_disabled: false,
				clicked: true,
				button_text: clean(sendBtn.textContent || ''),
				button_class: (sendBtn.className || '').toString(),
			});
		}`, msg).String()

		if err := json.Unmarshal([]byte(raw), &sendResult); err != nil {
			lastSendErr = fmt.Errorf("unmarshal send attempt failed: %w", err)
			time.Sleep(450 * time.Millisecond)
			continue
		}

		if !sendResult.InputFound {
			return nil, fmt.Errorf("message input box not found")
		}

		if sendResult.Clicked {
			sentTriggered = true
			break
		}

		// Some clients allow Enter to send; keep as a fallback path.
		_ = pp.Keyboard.Press(input.Enter)
		time.Sleep(600 * time.Millisecond)
	}

	if !sentTriggered && lastSendErr != nil {
		return nil, lastSendErr
	}

	confirmed := false
	for i := 0; i < 12; i++ {
		afterSnap, snapErr := readSnapshot(msg)
		if snapErr == nil {
			grew := afterSnap.Count > beforeSnap.Count
			moreMatches := afterSnap.TailMatches > beforeSnap.TailMatches
			lastMatched := strings.Contains(afterSnap.Last, msg)
			if (grew && (moreMatches || lastMatched)) || moreMatches {
				confirmed = true
				break
			}
		}
		time.Sleep(650 * time.Millisecond)
	}

	if !confirmed {
		return nil, fmt.Errorf("message might not be sent, no outgoing confirmation found")
	}

	if verifyLimit <= 0 || verifyLimit > 500 {
		verifyLimit = 30
	}
	return a.GetConversationByUsername(ctx, clickedName, verifyLimit)
}

func waitIMConversationReady(pp *rod.Page, expectedUsername string) bool {
	target := strings.TrimSpace(expectedUsername)
	for i := 0; i < 12; i++ {
		ready := pp.MustEval(`(expected) => {
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const topbarName = clean(document.querySelector('div[class*="message-topbar"] span[class*="text1"]')?.textContent || '');
			if (!topbarName) return false;
			if (!expected) return true;
			return topbarName.includes(expected) || expected.includes(topbarName);
		}`, target).Bool()
		if ready {
			return true
		}
		time.Sleep(400 * time.Millisecond)
	}
	return false
}

func waitIMConversationListReady(pp *rod.Page) bool {
	for i := 0; i < 20; i++ {
		ready := pp.MustEval(`() => {
			const count = document.querySelectorAll('div[class*="conversation-item"]').length;
			return count > 0;
		}`).Bool()
		if ready {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func findConversationByUsername(pp *rod.Page, username string) string {
	return pp.MustEval(`(target) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const normalize = (s) => clean(s).toLowerCase().replace(/[^0-9a-z\u4e00-\u9fa5]/g, '');
		const extractName = (item) => {
			let lines = (item.innerText || '').split('\n').map((s) => clean(s)).filter(Boolean);
			if (lines.length === 0) return '';
			if (/^\d+$/.test(lines[0])) lines = lines.slice(1);
			return lines[0] || '';
		};

		const normalizedTarget = normalize(target);
		const items = Array.from(document.querySelectorAll('div[class*="conversation-item"]'));

		// Phase 1: exact username match.
		for (const item of items) {
			const name = extractName(item);
			if (!name || name.includes('通知消息')) continue;
			const normalizedName = normalize(name);
			if (normalizedName !== normalizedTarget) continue;
			const clickable = item.querySelector('.ant-dropdown-trigger') || item.firstElementChild || item;
			if (clickable) {
				clickable.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
				clickable.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
				clickable.dispatchEvent(new MouseEvent('click', { bubbles: true }));
			}
			return name;
		}

		// Phase 2: tolerant contains match on username only.
		for (const item of items) {
			const name = extractName(item);
			if (!name || name.includes('通知消息')) continue;
			const normalizedName = normalize(name);
			if (
				normalizedName.includes(normalizedTarget) ||
				normalizedTarget.includes(normalizedName)
			) {
				const clickable = item.querySelector('.ant-dropdown-trigger') || item.firstElementChild || item;
				if (clickable) {
					clickable.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
					clickable.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
					clickable.dispatchEvent(new MouseEvent('click', { bubbles: true }));
				}
				return name;
			}
		}
		return '';
	}`, username).String()
}
