package xianyu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

type SearchAction struct {
	page *rod.Page
}

type SearchItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Price     string `json:"price,omitempty"`
	WantCount int    `json:"want_count,omitempty"`
	URL       string `json:"url"`
	Seller    string `json:"seller,omitempty"`
}

func NewSearchAction(page *rod.Page) *SearchAction {
	return &SearchAction{
		page: page.Timeout(60 * time.Second),
	}
}

func (a *SearchAction) Search(ctx context.Context, keyword string, limit int) ([]SearchItem, error) {
	pp := a.page.Context(ctx)
	targetURL := fmt.Sprintf("https://www.goofish.com/search?q=%s", url.QueryEscape(keyword))
	pp.MustNavigate(targetURL).MustWaitLoad()

	itemCount := 0
	for i := 0; i < 20; i++ {
		itemCount = pp.MustEval(`() => document.querySelectorAll('a[href*="/item?id="]').length`).Int()
		if itemCount > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	raw := pp.MustEval(`(limit) => {
		const items = [];
		const seen = new Set();
		const anchors = Array.from(document.querySelectorAll('a[href*="/item?id="]'));

		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();

		for (const a of anchors) {
			const href = a.href || a.getAttribute('href') || '';
			const idMatch = href.match(/[?&]id=(\d+)/);
			if (!idMatch) continue;
			const id = idMatch[1];
			if (seen.has(id)) continue;
			seen.add(id);

			const card = a.closest('[class*="card"]') || a.closest('article') || a.closest('li') || a.parentElement || a;
			const directText = clean(a.innerText || '');
			const cardText = clean(card.innerText || '');
			const text = directText || cardText;
			if (!text) continue;

			const priceMatch = text.match(/¥\s*([0-9]+(?:\.[0-9]+)?)/);
			const wantMatch = text.match(/([0-9]+)\s*人想要/);

			let title = text;
			if (priceMatch && priceMatch[0]) {
				const idx = title.indexOf(priceMatch[0]);
				if (idx > 0) title = title.slice(0, idx).trim();
			}
			title = title.replace(/\d+\s*人想要.*/g, '').trim();
			title = title.replace(/\d+分钟前发布.*/g, '').trim();
			if (!title) {
				const lines = text.split(' ');
				title = lines.slice(0, 20).join(' ');
			}

			let seller = '';
			const sellerMatch = text.match(/([^\s]{2,20})\s*卖家信用/);
			if (sellerMatch) seller = sellerMatch[1];

			items.push({
				id,
				title: title.slice(0, 100),
				price: priceMatch ? priceMatch[1] : '',
				want_count: wantMatch ? parseInt(wantMatch[1], 10) : 0,
				url: href,
				seller,
			});

			if (items.length >= limit) break;
		}

		return JSON.stringify(items);
	}`, limit).String()

	var items []SearchItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("unmarshal search result failed: %w", err)
	}

	if len(items) == 0 {
		bodyHead := pp.MustEval(`() => (document.body?.innerText || '').replace(/\s+/g, ' ').slice(0, 200)`).String()
		if strings.Contains(bodyHead, "验证") || strings.Contains(bodyHead, "安全") {
			return nil, fmt.Errorf("search page requires verification: %s", bodyHead)
		}
	}

	return items, nil
}
