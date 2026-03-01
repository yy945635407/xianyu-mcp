package xianyu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

type CommunityServiceAction struct {
	page *rod.Page
}

type CommunityFeedItem struct {
	Title    string `json:"title"`
	Price    string `json:"price,omitempty"`
	URL      string `json:"url,omitempty"`
	Seller   string `json:"seller,omitempty"`
	WantText string `json:"want_text,omitempty"`
	RawText  string `json:"raw_text,omitempty"`
}

type CommunityFeedResult struct {
	Keyword    string              `json:"keyword,omitempty"`
	Categories []string            `json:"categories,omitempty"`
	Count      int                 `json:"count"`
	Items      []CommunityFeedItem `json:"items"`
}

type CommunityActionResult struct {
	Success        bool   `json:"success"`
	Action         string `json:"action"`
	Message        string `json:"message"`
	Target         string `json:"target,omitempty"`
	RedirectURL    string `json:"redirect_url,omitempty"`
	RequiresManual bool   `json:"requires_manual,omitempty"`
}

type CustomerServiceEntry struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Source string `json:"source,omitempty"`
}

type AfterSaleRecord struct {
	Title   string   `json:"title,omitempty"`
	Status  string   `json:"status,omitempty"`
	Price   string   `json:"price,omitempty"`
	Seller  string   `json:"seller,omitempty"`
	Actions []string `json:"actions,omitempty"`
	RawText string   `json:"raw_text,omitempty"`
}

type CustomerServiceResult struct {
	Entries    []CustomerServiceEntry `json:"entries,omitempty"`
	AfterSales []AfterSaleRecord      `json:"after_sales,omitempty"`
}

func NewCommunityServiceAction(page *rod.Page) *CommunityServiceAction {
	return &CommunityServiceAction{page: page.Timeout(60 * time.Second)}
}

func (a *CommunityServiceAction) GetCommunityFeed(ctx context.Context, keyword string, limit int) (*CommunityFeedResult, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(homeURL).MustWaitLoad()
	time.Sleep(1500 * time.Millisecond)

	raw := pp.MustEval(`(keyword, limit) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const normalize = (s) => clean(s).toLowerCase().replace(/[^0-9a-z\u4e00-\u9fa5]/g, '');
		const kw = normalize(keyword || '');
		const toAbsURL = (href) => {
			try { return new URL(href, location.origin).toString(); } catch { return href || ''; }
		};

		const categories = Array.from(document.querySelectorAll('a[href], div, span'))
			.map((el) => clean(el.textContent || ''))
			.filter((t) => t && t.length >= 2 && t.length <= 12)
			.filter((t) => /手机|数码|电脑|服饰|箱包|运动|母婴|美妆|家电|家装|文玩|珠宝|图书|游戏|汽车|电动车|租房|宠物/.test(t))
			.filter((t, i, arr) => arr.indexOf(t) === i)
			.slice(0, 30);

		const links = Array.from(document.querySelectorAll('a[href*="/item?id="], a[href*="item?id="]'));
		const out = [];
		const seen = new Set();
		for (const a of links) {
			const href = toAbsURL(a.getAttribute('href') || a.href || '');
			if (!href) continue;
			if (seen.has(href)) continue;
			seen.add(href);
			const card = a.closest('div[class*="feeds-item-wrap"], div[class*="card"], li, article') || a.parentElement || a;
			const raw = clean((card?.innerText || a.innerText || '')).slice(0, 200);
			if (!raw) continue;
			const nraw = normalize(raw);
			if (kw && !nraw.includes(kw)) continue;
			const title = raw
				.replace(/¥\s*[0-9]+(?:\.[0-9]+)?/g, ' ')
				.replace(/\d+人想要/g, ' ')
				.replace(/\d+浏览/g, ' ')
				.replace(/\s+/g, ' ')
				.trim()
				.slice(0, 80);
			const finalTitle = title || raw.slice(0, 60);
			const price = clean((raw.match(/¥\s*[0-9]+(?:\.[0-9]+)?/) || [])[0] || '');
			const wantText = clean((raw.match(/\d+人想要/) || [])[0] || '');
			out.push({
				title: finalTitle,
				price,
				url: href,
				want_text: wantText,
				raw_text: raw,
			});
			if (out.length >= limit) break;
		}

		return JSON.stringify({
			keyword: clean(keyword),
			categories,
			count: out.length,
			items: out,
		});
	}`, keyword, limit).String()

	var result CommunityFeedResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("unmarshal community feed failed: %w", err)
	}
	return &result, nil
}

func (a *CommunityServiceAction) InteractCommunity(ctx context.Context, keyword, action string) (*CommunityActionResult, error) {
	kw := strings.TrimSpace(keyword)
	if kw == "" {
		return nil, fmt.Errorf("keyword is required")
	}

	mode := strings.ToLower(strings.TrimSpace(action))
	if mode == "" {
		mode = "open_item"
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(homeURL).MustWaitLoad()
	time.Sleep(1200 * time.Millisecond)

	raw := pp.MustEval(`(keyword, mode) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const normalize = (s) => clean(s).toLowerCase().replace(/[^0-9a-z\u4e00-\u9fa5]/g, '');
		const kw = normalize(keyword);
		const toAbsURL = (href) => {
			try { return new URL(href, location.origin).toString(); } catch { return href || ''; }
		};

		if (mode === 'open_category') {
			const nodes = Array.from(document.querySelectorAll('a,div,span,button'));
			const node = nodes.find((el) => {
				const t = clean(el.textContent || '');
				if (!t || t.length > 16) return false;
				return normalize(t).includes(kw);
			});
			if (!node) return JSON.stringify({ ok: false, msg: '未找到匹配分类入口', target: keyword });
			node.click();
			return JSON.stringify({ ok: true, msg: '已触发分类入口', target: clean(node.textContent || ''), url: location.href });
		}

		const links = Array.from(document.querySelectorAll('a[href*="/item?id="], a[href*="item?id="]'));
		for (const a of links) {
			const txt = clean((a.closest('div[class*="feeds-item-wrap"], div[class*="card"], li, article') || a).innerText || a.innerText || '');
			if (!normalize(txt).includes(kw)) continue;
			const href = toAbsURL(a.getAttribute('href') || a.href || '');
			a.click();
			return JSON.stringify({ ok: true, msg: '已打开社区推荐商品', target: txt.slice(0, 60), url: href || location.href });
		}
		return JSON.stringify({ ok: false, msg: '未找到匹配社区内容', target: keyword });
	}`, kw, mode).String()

	type resp struct {
		OK     bool   `json:"ok"`
		Msg    string `json:"msg"`
		Target string `json:"target"`
		URL    string `json:"url"`
	}
	var r resp
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, fmt.Errorf("parse community interaction result failed: %w", err)
	}

	return &CommunityActionResult{
		Success:     r.OK,
		Action:      mode,
		Message:     r.Msg,
		Target:      r.Target,
		RedirectURL: r.URL,
	}, nil
}

func (a *CommunityServiceAction) GetCustomerService(ctx context.Context, aftersaleLimit int) (*CustomerServiceResult, error) {
	if aftersaleLimit <= 0 || aftersaleLimit > 100 {
		aftersaleLimit = 20
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(homeURL).MustWaitLoad()
	time.Sleep(1000 * time.Millisecond)

	entriesRaw := pp.MustEval(`() => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const toAbsURL = (href) => {
			try { return new URL(href, location.origin).toString(); } catch { return href || ''; }
		};
		const links = Array.from(document.querySelectorAll('a[href]'));
		const out = [];
		for (const a of links) {
			const text = clean(a.textContent || '');
			if (!text) continue;
			if (!(text.includes('客服') || text.includes('反馈'))) continue;
			const href = toAbsURL(a.getAttribute('href') || a.href || '');
			if (!href) continue;
			out.push({ name: text, url: href, source: 'sidebar' });
		}
		if (out.length === 0) {
			const text = clean(document.body?.innerText || '');
			if (text.includes('客服')) {
				out.push({ name: '客服', url: '', source: 'sidebar' });
			}
		}
		return JSON.stringify(out.filter((x, i, arr) => arr.findIndex((y) => y.name === x.name && y.url === x.url) === i));
	}`).String()

	var entries []CustomerServiceEntry
	if err := json.Unmarshal([]byte(entriesRaw), &entries); err != nil {
		return nil, fmt.Errorf("unmarshal customer service entries failed: %w", err)
	}

	orders, err := a.listAfterSales(pp, aftersaleLimit)
	if err != nil {
		return nil, err
	}

	return &CustomerServiceResult{
		Entries:    entries,
		AfterSales: orders,
	}, nil
}

func (a *CommunityServiceAction) OpenCustomerService(ctx context.Context, name string) (*CommunityActionResult, error) {
	target := strings.TrimSpace(name)
	if target == "" {
		target = "客服"
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(homeURL).MustWaitLoad()
	time.Sleep(1000 * time.Millisecond)

	raw := pp.MustEval(`(target) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const links = Array.from(document.querySelectorAll('a[href]'));
		for (const a of links) {
			const t = clean(a.textContent || '');
			if (!t) continue;
			if (!(t.includes(target) || (target === '客服' && t.includes('反馈')))) continue;
			const href = a.getAttribute('href') || a.href || '';
			a.click();
			return JSON.stringify({ ok: true, msg: '已打开入口', target: t, url: href });
		}
		return JSON.stringify({ ok: false, msg: '未找到客服/反馈入口', target });
	}`, target).String()

	type openResp struct {
		OK     bool   `json:"ok"`
		Msg    string `json:"msg"`
		Target string `json:"target"`
		URL    string `json:"url"`
	}
	var resp openResp
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("parse open customer service result failed: %w", err)
	}

	return &CommunityActionResult{
		Success:     resp.OK,
		Action:      "open_customer_service",
		Message:     resp.Msg,
		Target:      resp.Target,
		RedirectURL: resp.URL,
	}, nil
}

func (a *CommunityServiceAction) listAfterSales(pp *rod.Page, limit int) ([]AfterSaleRecord, error) {
	pp.MustNavigate(boughtURL).MustWaitLoad()
	time.Sleep(1000 * time.Millisecond)
	_ = switchOrderTab(pp, "退款中")
	time.Sleep(1200 * time.Millisecond)
	if !waitOrderListReady(pp) {
		return nil, fmt.Errorf("after-sale order list not ready")
	}

	raw := pp.MustEval(`(limit) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const cards = Array.from(document.querySelectorAll('div[class*="container--Bhfvcld8"]'));
		const out = [];
		for (const card of cards) {
			const seller = clean(card.querySelector('div[class*="name--"] span')?.textContent || '');
			const status = clean(card.querySelector('div[class*="status--"] span')?.textContent || '');
			const title = clean(card.querySelector('div[class*="desc--"]')?.textContent || '');
			const price = clean(card.innerText.match(/¥\s*[0-9]+(?:\.[0-9]+)?/)?.[0] || '');
			const actions = Array.from(card.querySelectorAll('button,div,a,span'))
				.map((el) => clean(el.textContent))
				.filter((t) => t && t.length <= 20)
				.filter((t) => ['退款详情', '投诉卖家', '联系卖家', '查看钱款', '宝贝快照', '删除订单', '更多'].some((k) => t.includes(k)));
			out.push({
				title,
				status,
				price,
				seller,
				actions,
				raw_text: clean(card.innerText).slice(0, 220),
			});
			if (out.length >= limit) break;
		}
		return JSON.stringify(out);
	}`, limit).String()

	var result []AfterSaleRecord
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("unmarshal after-sale list failed: %w", err)
	}
	return result, nil
}
