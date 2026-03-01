package xianyu

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

type ItemDetailAction struct {
	page *rod.Page
}

type ItemDetail struct {
	ItemID      string   `json:"item_id,omitempty"`
	ItemURL     string   `json:"item_url"`
	Title       string   `json:"title,omitempty"`
	Price       string   `json:"price,omitempty"`
	Original    string   `json:"original_price,omitempty"`
	Shipping    string   `json:"shipping,omitempty"`
	SellerName  string   `json:"seller_name,omitempty"`
	SellerID    string   `json:"seller_id,omitempty"`
	Location    string   `json:"location,omitempty"`
	WantCount   string   `json:"want_count,omitempty"`
	BrowseCount string   `json:"browse_count,omitempty"`
	Description string   `json:"description,omitempty"`
	ChatURL     string   `json:"chat_url,omitempty"`
	BuyURL      string   `json:"buy_url,omitempty"`
	Favorited   bool     `json:"favorited,omitempty"`
	Images      []string `json:"images,omitempty"`
	RawText     string   `json:"raw_text,omitempty"`
}

type ItemActionResult struct {
	Success        bool   `json:"success"`
	Action         string `json:"action"`
	Message        string `json:"message"`
	ItemURL        string `json:"item_url,omitempty"`
	RedirectURL    string `json:"redirect_url,omitempty"`
	RequiresManual bool   `json:"requires_manual,omitempty"`
}

func NewItemDetailAction(page *rod.Page) *ItemDetailAction {
	return &ItemDetailAction{page: page.Timeout(60 * time.Second)}
}

func (a *ItemDetailAction) GetDetail(ctx context.Context, itemRef string) (*ItemDetail, error) {
	itemURL, err := resolveItemURL(itemRef)
	if err != nil {
		return nil, err
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(itemURL).MustWaitLoad()
	if !waitItemPageReady(pp) {
		return nil, fmt.Errorf("item detail page not ready")
	}

	raw := pp.MustEval(`() => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const toAbsURL = (href) => {
			try { return new URL(href, location.origin).toString(); } catch { return href || ''; }
		};
		const body = clean(document.body?.innerText || '');
		const itemID = new URLSearchParams(location.search).get('id') || new URLSearchParams(location.search).get('itemId') || '';

		const sellerLink = document.querySelector('a[href*="personal?userId="], a[href*="userId="]');
		let sellerID = '';
		if (sellerLink) {
			const href = sellerLink.getAttribute('href') || '';
			const m = href.match(/userId=(\d+)/);
			if (m) sellerID = m[1];
		}

		const chatLink = document.querySelector('a[href*="/im?itemId="]');
		const buyLink = document.querySelector('a[href*="/create-order"], a[href*="create-order"]');
		const favoriteBtn = Array.from(document.querySelectorAll('button,div,span,a')).find((el) => {
			const t = clean(el.textContent || '');
			return t === '收藏' || t === '已收藏' || t.includes('收藏');
		});
		const favText = clean(favoriteBtn?.textContent || '');

		const images = Array.from(document.querySelectorAll('img[src]'))
			.map((img) => img.getAttribute('src') || '')
			.map((src) => toAbsURL(src))
			.filter((src) => src && /^https?:\/\//.test(src))
			.filter((src) => !src.includes('avatar') && !src.includes('logo'))
			.filter((src, idx, arr) => arr.indexOf(src) === idx)
			.slice(0, 12);

		const title = clean(document.querySelector('h1,h2,[class*="title"],[class*="desc"]')?.textContent || '');
		const price = clean((body.match(/¥\s*[0-9]+(?:\.[0-9]+)?/) || [])[0] || '');
		const originalPrice = clean((body.match(/原价\s*¥\s*[0-9]+(?:\.[0-9]+)?/) || [])[0] || '').replace('原价', '').trim();
		const loc = clean((body.match(/(北京|上海|天津|重庆|河北|山西|辽宁|吉林|黑龙江|江苏|浙江|安徽|福建|江西|山东|河南|湖北|湖南|广东|海南|四川|贵州|云南|陕西|甘肃|青海|台湾|内蒙古|广西|西藏|宁夏|新疆|香港|澳门)\S*/) || [])[0] || '');
		const want = clean((body.match(/\d+人想要/) || [])[0] || '');
		const browse = clean((body.match(/\d+浏览/) || [])[0] || '');
		const shipping = clean((body.match(/包邮|不包邮|运费\S*/) || [])[0] || '');

		const sellerName = clean(document.querySelector('[class*="item-user-info-name"], [class*="nick"], [class*="user"]')?.textContent || '');

		return JSON.stringify({
			item_id: itemID,
			item_url: location.href,
			title,
			price,
			original_price: originalPrice,
			shipping,
			seller_name: sellerName,
			seller_id: sellerID,
			location: loc,
			want_count: want,
			browse_count: browse,
			description: body.slice(0, 400),
			chat_url: chatLink ? toAbsURL(chatLink.getAttribute('href') || chatLink.href || '') : '',
			buy_url: buyLink ? toAbsURL(buyLink.getAttribute('href') || buyLink.href || '') : '',
			favorited: favText.includes('已收藏'),
			images,
			raw_text: body.slice(0, 600),
		});
	}`).String()

	var detail ItemDetail
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		return nil, fmt.Errorf("unmarshal item detail failed: %w", err)
	}
	if detail.ItemURL == "" {
		detail.ItemURL = itemURL
	}
	if detail.ItemID == "" {
		detail.ItemID = parseItemRefID(itemURL)
	}
	return &detail, nil
}

func (a *ItemDetailAction) Favorite(ctx context.Context, itemRef string) (*ItemActionResult, error) {
	itemURL, err := resolveItemURL(itemRef)
	if err != nil {
		return nil, err
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(itemURL).MustWaitLoad()
	if !waitItemPageReady(pp) {
		return nil, fmt.Errorf("item detail page not ready")
	}

	raw := pp.MustEval(`() => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const cands = Array.from(document.querySelectorAll('button,div,span,a'));
		const btn = cands.find((el) => {
			const t = clean(el.textContent || '');
			if (!t || t.length > 10) return false;
			return t === '收藏' || t === '已收藏' || t.includes('收藏');
		});
		if (!btn) return JSON.stringify({ ok: false, msg: '未找到收藏按钮' });
		const before = clean(btn.textContent || '');
		btn.click();
		const after = clean(btn.textContent || '');
		return JSON.stringify({ ok: true, msg: '已触发收藏动作', before, after, url: location.href });
	}`).String()

	type favResp struct {
		OK  bool   `json:"ok"`
		Msg string `json:"msg"`
		URL string `json:"url"`
	}
	var resp favResp
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("parse favorite result failed: %w", err)
	}
	return &ItemActionResult{
		Success: resp.OK,
		Action:  "favorite",
		Message: resp.Msg,
		ItemURL: resp.URL,
	}, nil
}

func (a *ItemDetailAction) Chat(ctx context.Context, itemRef, message string) (*ItemActionResult, error) {
	detail, err := a.GetDetail(ctx, itemRef)
	if err != nil {
		return nil, err
	}
	if detail.ChatURL == "" {
		return &ItemActionResult{
			Success:        false,
			Action:         "chat",
			Message:        "未找到聊一聊入口",
			ItemURL:        detail.ItemURL,
			RequiresManual: true,
		}, nil
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(detail.ChatURL).MustWaitLoad()
	time.Sleep(1200 * time.Millisecond)

	msg := strings.TrimSpace(message)
	if msg == "" {
		return &ItemActionResult{
			Success:     true,
			Action:      "chat",
			Message:     "已打开聊一聊会话",
			ItemURL:     detail.ItemURL,
			RedirectURL: detail.ChatURL,
		}, nil
	}

	sent := pp.MustEval(`(text) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const input = document.querySelector('textarea[placeholder*="请输入消息"], textarea[class*="ant-input"], textarea');
		if (!input) return JSON.stringify({ok:false,msg:'未找到消息输入框'});
		input.focus();
		const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value')?.set;
		if (setter) {
			setter.call(input, text);
			input.dispatchEvent(new Event('input', { bubbles: true }));
		} else {
			input.value = text;
			input.dispatchEvent(new Event('input', { bubbles: true }));
		}
		input.dispatchEvent(new Event('change', { bubbles: true }));

		const btn = Array.from(document.querySelectorAll('div[class*="sendbox-bottom"] button, button')).find((el) => clean(el.textContent || '').replace(/\s+/g,'') === '发送');
		if (btn && !btn.disabled) {
			btn.click();
			return JSON.stringify({ok:true,msg:'已发送消息'});
		}
		return JSON.stringify({ok:false,msg:'未找到可用发送按钮'});
	}`, msg).String()

	type sendResp struct {
		OK  bool   `json:"ok"`
		Msg string `json:"msg"`
	}
	var resp sendResp
	_ = json.Unmarshal([]byte(sent), &resp)

	return &ItemActionResult{
		Success:     resp.OK,
		Action:      "chat",
		Message:     resp.Msg,
		ItemURL:     detail.ItemURL,
		RedirectURL: detail.ChatURL,
	}, nil
}

func (a *ItemDetailAction) Buy(ctx context.Context, itemRef string) (*ItemActionResult, error) {
	detail, err := a.GetDetail(ctx, itemRef)
	if err != nil {
		return nil, err
	}
	if detail.BuyURL == "" {
		return &ItemActionResult{
			Success:        false,
			Action:         "buy",
			Message:        "未找到立即购买入口",
			ItemURL:        detail.ItemURL,
			RequiresManual: true,
		}, nil
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(detail.ItemURL).MustWaitLoad()
	time.Sleep(900 * time.Millisecond)

	raw := pp.MustEval(`() => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const buyLink = document.querySelector('a[href*="/create-order"], a[href*="create-order"]');
		if (buyLink) {
			const href = buyLink.getAttribute('href') || buyLink.href || '';
			buyLink.click();
			return JSON.stringify({ok:true,url:href,msg:'已触发立即购买'});
		}
		const btn = Array.from(document.querySelectorAll('button,div,span,a')).find((el) => {
			const t = clean(el.textContent || '');
			return t === '立即购买' || t.includes('立即购买');
		});
		if (!btn) return JSON.stringify({ok:false,url:'',msg:'未找到立即购买按钮'});
		btn.click();
		return JSON.stringify({ok:true,url:location.href,msg:'已触发立即购买'});
	}`).String()

	type buyResp struct {
		OK  bool   `json:"ok"`
		URL string `json:"url"`
		Msg string `json:"msg"`
	}
	var resp buyResp
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("parse buy result failed: %w", err)
	}

	if resp.URL == "" {
		resp.URL = detail.BuyURL
	}

	return &ItemActionResult{
		Success:     resp.OK,
		Action:      "buy",
		Message:     resp.Msg,
		ItemURL:     detail.ItemURL,
		RedirectURL: resp.URL,
	}, nil
}

func resolveItemURL(itemRef string) (string, error) {
	ref := strings.TrimSpace(itemRef)
	if ref == "" {
		return "", fmt.Errorf("item_ref is required")
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref, nil
	}
	id := parseItemRefID(ref)
	if id == "" {
		return "", fmt.Errorf("invalid item_ref: %s", ref)
	}
	return fmt.Sprintf("https://www.goofish.com/item?id=%s", id), nil
}

func parseItemRefID(ref string) string {
	clean := strings.TrimSpace(ref)
	if clean == "" {
		return ""
	}
	if regexp.MustCompile(`^[0-9]{8,}$`).MatchString(clean) {
		return clean
	}
	idReg := regexp.MustCompile(`(?:id|itemId)=([0-9]+)`)
	if m := idReg.FindStringSubmatch(clean); len(m) == 2 {
		return m[1]
	}
	return ""
}

func waitItemPageReady(pp *rod.Page) bool {
	for i := 0; i < 20; i++ {
		ready := pp.MustEval(`() => {
			const txt = (document.body?.innerText || '').replace(/\s+/g, ' ');
			const hasTitle = !!document.querySelector('h1,h2,[class*="title"],[class*="desc"]');
			const hasPrice = /¥\s*[0-9]+/.test(txt);
			return hasTitle && hasPrice;
		}`).Bool()
		if ready {
			return true
		}
		time.Sleep(400 * time.Millisecond)
	}
	return false
}
