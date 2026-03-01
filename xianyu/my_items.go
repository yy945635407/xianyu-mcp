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

const personalURL = "https://www.goofish.com/personal"

type MyItemsAction struct {
	page *rod.Page
}

type MyItemSummary struct {
	Title    string `json:"title"`
	Price    string `json:"price,omitempty"`
	URL      string `json:"url"`
	ItemID   string `json:"item_id,omitempty"`
	Status   string `json:"status,omitempty"`
	Seller   string `json:"seller,omitempty"`
	RawText  string `json:"raw_text,omitempty"`
	WantText string `json:"want_text,omitempty"`
}

type MyItemListResult struct {
	Tab   string          `json:"tab"`
	Count int             `json:"count"`
	Items []MyItemSummary `json:"items"`
}

type MyItemActionResult struct {
	Success        bool   `json:"success"`
	Action         string `json:"action"`
	Message        string `json:"message"`
	ItemTitle      string `json:"item_title,omitempty"`
	ItemURL        string `json:"item_url,omitempty"`
	RequiresManual bool   `json:"requires_manual,omitempty"`
}

type EditMyItemParams struct {
	Keyword     string
	ItemRef     string
	Tab         string
	Price       string
	Description string
	Submit      bool
}

func NewMyItemsAction(page *rod.Page) *MyItemsAction {
	return &MyItemsAction{page: page.Timeout(60 * time.Second)}
}

func (a *MyItemsAction) ListMyItems(ctx context.Context, tab string, limit int) (*MyItemListResult, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(personalURL).MustWaitLoad()
	if !waitMyItemsReady(pp) {
		return nil, fmt.Errorf("personal page not ready")
	}
	_ = pp.MustEval(`() => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const nodes = Array.from(document.querySelectorAll('div,span,a,button'));
		const tab = nodes.find((el) => {
			const t = clean(el.textContent || '');
			return t === '宝贝' || t.includes('宝贝');
		});
		if (tab) tab.click();
	}`)
	time.Sleep(500 * time.Millisecond)

	selectedTab := normalizeMyTab(tab)
	if selectedTab != "" && selectedTab != "在售" {
		_ = switchMyItemsTab(pp, selectedTab)
		time.Sleep(1000 * time.Millisecond)
	}

	extract := func() (string, error) {
		return pp.MustEval(`(limit, tab) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const toAbsURL = (href) => {
			try {
				return new URL(href, location.origin).toString();
			} catch {
				return href || '';
			}
		};
		const parseID = (href) => {
			try {
				const u = new URL(href, location.origin);
				return u.searchParams.get('id') || u.searchParams.get('itemId') || '';
			} catch {
				const m = (href || '').match(/(?:id|itemId)=([0-9]+)/);
				return m ? m[1] : '';
			}
		};
		const norm = (s) => clean(s).toLowerCase().replace(/[^0-9a-z\u4e00-\u9fa5]/g, '');

		let currentTab = clean(tab || '');
		if (!currentTab) {
			const active = Array.from(document.querySelectorAll('div[class*="tabItem"], div[class*="textSelected"], div[class*="selected"]'))
				.map((el) => clean(el.textContent || ''))
				.find((t) => /在售|已售|下架|古月/.test(t));
			currentTab = active || '在售';
		}

		const items = [];
		const seen = new Set();
		const links = Array.from(document.querySelectorAll('a[href*="/item?id="], a[href*="item?id="], a[href*="itemId="]'));
		for (const a of links) {
			const href = toAbsURL(a.getAttribute('href') || a.href || '');
			if (!href) continue;
			const id = parseID(href);
			const key = String(id || '') + '::' + href;
			if (seen.has(key)) continue;
			seen.add(key);

			const card = a.closest('div[class*="feeds-item-wrap"], li[class*="item"], article') || a.parentElement || a;
			const anchorText = clean(a.innerText || '');
			let raw = clean(card.innerText || anchorText || '');
			if (raw.length > 220 && anchorText) raw = anchorText;
			if (!raw) continue;

			let title = raw
				.replace(/¥\s*[0-9]+(?:\.[0-9]+)?/g, ' ')
				.replace(/可小刀/g, ' ')
				.replace(/\s+/g, ' ')
				.trim();
			if (!title || title.length > 100) {
				title = clean(a.getAttribute('title') || a.textContent || raw).slice(0, 80);
			}
			const priceMatch = raw.match(/¥\s*[0-9]+(?:\.[0-9]+)?/);
			const price = priceMatch ? clean(priceMatch[0]) : '';
			const wantText = clean((raw.match(/[0-9]+人想要/) || [])[0] || '');
			items.push({
				title,
				price,
				url: href,
				item_id: id,
				status: currentTab,
				want_text: wantText,
				raw_text: raw.slice(0, 180),
			});
			if (items.length >= limit) break;
		}

		return JSON.stringify({
			tab: currentTab,
			count: items.length,
			items,
		});
	}`, limit, selectedTab).String(), nil
	}

	var result MyItemListResult
	var lastErr error
	for i := 0; i < 4; i++ {
		raw, err := extract()
		if err != nil {
			lastErr = err
			time.Sleep(800 * time.Millisecond)
			continue
		}
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			lastErr = fmt.Errorf("unmarshal my items failed: %w", err)
			time.Sleep(800 * time.Millisecond)
			continue
		}
		if result.Count > 0 {
			break
		}
		time.Sleep(900 * time.Millisecond)
	}
	if lastErr != nil && result.Count == 0 {
		return nil, lastErr
	}
	result.Tab = normalizeMyTab(result.Tab)
	if result.Tab == "" {
		result.Tab = selectedTab
	}
	if result.Tab == "" {
		result.Tab = "在售"
	}
	return &result, nil
}

func (a *MyItemsAction) EditMyItem(ctx context.Context, req EditMyItemParams) (*MyItemActionResult, error) {
	itemURL, title, err := a.findMyItemURL(ctx, req.Keyword, req.ItemRef, req.Tab)
	if err != nil {
		return nil, err
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(itemURL).MustWaitLoad()
	time.Sleep(1200 * time.Millisecond)

	clickedEdit := pp.MustEval(`() => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const cands = Array.from(document.querySelectorAll('button,div,span,a'));
		const btn = cands.find((el) => {
			const t = clean(el.textContent || '');
			if (!t || t.length > 14) return false;
			return t === '编辑' || t.includes('编辑宝贝') || t.includes('编辑');
		});
		if (!btn) return false;
		btn.click();
		return true;
	}`).Bool()
	if !clickedEdit {
		return &MyItemActionResult{
			Success:        false,
			Action:         "edit",
			Message:        "未找到编辑按钮，请手动在商品页操作",
			ItemTitle:      title,
			ItemURL:        itemURL,
			RequiresManual: true,
		}, nil
	}

	time.Sleep(2 * time.Second)
	fillRaw := pp.MustEval(`(price, description) => {
		const setInput = (el, val) => {
			if (!el) return false;
			el.focus();
			const proto = el.tagName.toLowerCase() === 'textarea' ? window.HTMLTextAreaElement.prototype : window.HTMLInputElement.prototype;
			const setter = Object.getOwnPropertyDescriptor(proto, 'value')?.set;
			if (setter) {
				setter.call(el, '');
				el.dispatchEvent(new Event('input', { bubbles: true }));
				setter.call(el, val);
				el.dispatchEvent(new Event('input', { bubbles: true }));
			} else {
				el.value = val;
				el.dispatchEvent(new Event('input', { bubbles: true }));
			}
			el.dispatchEvent(new Event('change', { bubbles: true }));
			return true;
		};

		let priceFilled = false;
		if ((price || '').trim()) {
			const priceInput = document.querySelector('input[placeholder*="价格"], input[placeholder*="售价"], input[name*="price"], input[type="number"]');
			priceFilled = setInput(priceInput, String(price).trim());
		}

		let descFilled = false;
		if ((description || '').trim()) {
			const descInput = document.querySelector('textarea[placeholder*="描述"], textarea[placeholder*="宝贝"], textarea, [contenteditable="true"]');
			if (descInput) {
				if (descInput.tagName.toLowerCase() === 'textarea') {
					descFilled = setInput(descInput, String(description).trim());
				} else {
					descInput.textContent = String(description).trim();
					descInput.dispatchEvent(new Event('input', { bubbles: true }));
					descInput.dispatchEvent(new Event('change', { bubbles: true }));
					descFilled = true;
				}
			}
		}

		return JSON.stringify({price_filled: priceFilled, desc_filled: descFilled, url: location.href});
	}`, req.Price, req.Description).String()

	type fillResp struct {
		PriceFilled bool   `json:"price_filled"`
		DescFilled  bool   `json:"desc_filled"`
		URL         string `json:"url"`
	}
	var fr fillResp
	_ = json.Unmarshal([]byte(fillRaw), &fr)

	if req.Submit {
		_ = pp.MustEval(`() => {
			const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
			const cands = Array.from(document.querySelectorAll('button,div,span,a'));
			const btn = cands.find((el) => {
				const t = clean(el.textContent || '');
				return t === '保存' || t === '确认' || t === '完成' || t === '发布' || t.includes('保存修改');
			});
			if (btn) btn.click();
		}`)
		time.Sleep(800 * time.Millisecond)
	}

	msg := "已进入编辑流程"
	if req.Submit {
		msg = "已尝试提交编辑"
	}
	return &MyItemActionResult{
		Success:   true,
		Action:    "edit",
		Message:   msg,
		ItemTitle: title,
		ItemURL:   fr.URL,
	}, nil
}

func (a *MyItemsAction) ShelfMyItem(ctx context.Context, keyword, itemRef, tab, action string) (*MyItemActionResult, error) {
	itemURL, title, err := a.findMyItemURL(ctx, keyword, itemRef, tab)
	if err != nil {
		return nil, err
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(itemURL).MustWaitLoad()
	time.Sleep(1200 * time.Millisecond)

	actionText := strings.TrimSpace(action)
	if actionText == "" {
		actionText = "auto"
	}

	raw := pp.MustEval(`(mode) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const cands = Array.from(document.querySelectorAll('button,div,span,a'));
		let keywords = [];
		if (mode === 'up' || mode === '上架') {
			keywords = ['上架', '重新上架'];
		} else if (mode === 'down' || mode === '下架') {
			keywords = ['下架'];
		} else {
			keywords = ['下架', '上架', '重新上架'];
		}

		for (const key of keywords) {
			const btn = cands.find((el) => {
				const t = clean(el.textContent || '');
				if (!t || t.length > 12) return false;
				return t === key || t.includes(key);
			});
			if (btn) {
				btn.click();
				const confirm = cands.find((el) => {
					const t = clean(el.textContent || '');
					return t === '确认' || t === '确定' || t.includes('继续');
				});
				if (confirm) confirm.click();
				return JSON.stringify({ ok: true, clicked: clean(btn.textContent || '') });
			}
		}
		return JSON.stringify({ ok: false, clicked: '' });
	}`, actionText).String()

	type shelfResp struct {
		OK      bool   `json:"ok"`
		Clicked string `json:"clicked"`
	}
	var resp shelfResp
	_ = json.Unmarshal([]byte(raw), &resp)
	if !resp.OK {
		return &MyItemActionResult{
			Success:        false,
			Action:         "shelf",
			Message:        "未找到上架/下架按钮，请手动处理",
			ItemTitle:      title,
			ItemURL:        itemURL,
			RequiresManual: true,
		}, nil
	}

	return &MyItemActionResult{
		Success:   true,
		Action:    "shelf",
		Message:   "已触发“" + resp.Clicked + "”",
		ItemTitle: title,
		ItemURL:   itemURL,
	}, nil
}

func (a *MyItemsAction) DeleteMyItem(ctx context.Context, keyword, itemRef, tab string) (*MyItemActionResult, error) {
	itemURL, title, err := a.findMyItemURL(ctx, keyword, itemRef, tab)
	if err != nil {
		return nil, err
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(itemURL).MustWaitLoad()
	time.Sleep(1200 * time.Millisecond)

	raw := pp.MustEval(`() => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const cands = Array.from(document.querySelectorAll('button,div,span,a'));
		const del = cands.find((el) => {
			const t = clean(el.textContent || '');
			if (!t || t.length > 12) return false;
			return t === '删除' || t.includes('删除宝贝');
		});
		if (!del) return JSON.stringify({ ok: false });
		del.click();

		setTimeout(() => {
			const confirmNodes = Array.from(document.querySelectorAll('button,div,span,a'));
			const confirm = confirmNodes.find((el) => {
				const t = clean(el.textContent || '');
				return t === '确认' || t === '确定' || t.includes('删除');
			});
			if (confirm) confirm.click();
		}, 50);
		return JSON.stringify({ ok: true });
	}`).String()

	type deleteResp struct {
		OK bool `json:"ok"`
	}
	var resp deleteResp
	_ = json.Unmarshal([]byte(raw), &resp)
	if !resp.OK {
		return &MyItemActionResult{
			Success:        false,
			Action:         "delete",
			Message:        "未找到删除按钮，请手动处理",
			ItemTitle:      title,
			ItemURL:        itemURL,
			RequiresManual: true,
		}, nil
	}

	return &MyItemActionResult{
		Success:   true,
		Action:    "delete",
		Message:   "已触发删除流程，请在页面确认结果",
		ItemTitle: title,
		ItemURL:   itemURL,
	}, nil
}

func (a *MyItemsAction) findMyItemURL(ctx context.Context, keyword, itemRef, tab string) (string, string, error) {
	ref := strings.TrimSpace(itemRef)
	kw := strings.TrimSpace(keyword)

	if ref != "" {
		if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
			return ref, "", nil
		}
	}

	list, err := a.ListMyItems(ctx, tab, 60)
	if err != nil {
		return "", "", err
	}
	if len(list.Items) == 0 {
		return "", "", fmt.Errorf("my items list is empty")
	}

	idRef := parseMyItemID(ref)
	for _, item := range list.Items {
		titleNorm := normalizeTextForMatch(item.Title)
		urlNorm := normalizeTextForMatch(item.URL)
		idNorm := normalizeTextForMatch(item.ItemID)

		if idRef != "" && idNorm == normalizeTextForMatch(idRef) {
			return item.URL, item.Title, nil
		}
		if ref != "" {
			nref := normalizeTextForMatch(ref)
			if strings.Contains(urlNorm, nref) || strings.Contains(idNorm, nref) || strings.Contains(titleNorm, nref) {
				return item.URL, item.Title, nil
			}
		}
		if kw != "" && strings.Contains(titleNorm, normalizeTextForMatch(kw)) {
			return item.URL, item.Title, nil
		}
	}

	if kw == "" && ref == "" {
		return list.Items[0].URL, list.Items[0].Title, nil
	}
	return "", "", fmt.Errorf("target item not found")
}

func waitMyItemsReady(pp *rod.Page) bool {
	for i := 0; i < 20; i++ {
		ready := pp.MustEval(`() => {
			const links = document.querySelectorAll('a[href*="/item?id="], a[href*="item?id="], a[href*="itemId="]').length;
			if (links > 0) return true;
			const txt = (document.body?.innerText || '').replace(/\s+/g, ' ');
			return txt.includes('我发布的') || txt.includes('宝贝管理') || txt.includes('暂无宝贝');
		}`).Bool()
		if ready {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func normalizeMyTab(tab string) string {
	t := strings.TrimSpace(tab)
	if t == "" {
		return ""
	}
	if strings.Contains(t, "在售") || strings.EqualFold(t, "onsale") || strings.EqualFold(t, "selling") {
		return "在售"
	}
	if strings.Contains(t, "已售") || strings.EqualFold(t, "sold") {
		return "已售出"
	}
	if strings.Contains(t, "下架") || strings.Contains(t, "古月") || strings.EqualFold(t, "off") || strings.EqualFold(t, "offshelf") {
		return "下架"
	}
	return t
}

func switchMyItemsTab(pp *rod.Page, tab string) bool {
	target := normalizeMyTab(tab)
	if target == "" {
		return false
	}
	return pp.MustEval(`(tab) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const normalize = (s) => clean(s).toLowerCase().replace(/[^0-9a-z\u4e00-\u9fa5]/g, '');
		const target = normalize(tab);
		const aliases = target === normalize('下架') ? [normalize('下架'), normalize('古月')] : [target];
		const nodes = Array.from(document.querySelectorAll('div,span,a,button'));
		for (const el of nodes) {
			const t = clean(el.textContent || '');
			if (!t || t.length > 20) continue;
			const nt = normalize(t);
			if (!aliases.some((a) => nt.includes(a) || a.includes(nt))) continue;
			if (!(nt.includes(normalize('在售')) || nt.includes(normalize('已售')) || nt.includes(normalize('下架')) || nt.includes(normalize('古月')))) continue;
			const visible = !!(el.offsetWidth || el.offsetHeight || el.getClientRects().length);
			if (!visible) continue;
			el.click();
			return true;
		}
		return false;
	}`, target).Bool()
}

func normalizeTextForMatch(v string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= 0x4e00 && r <= 0x9fa5) {
			return r
		}
		return -1
	}, strings.TrimSpace(v)))
}

func parseMyItemID(ref string) string {
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
