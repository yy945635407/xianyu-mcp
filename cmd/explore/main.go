package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/ylyt_bot/xianyu-mcp/browser"
)

type entry struct {
	Text string `json:"text"`
	Href string `json:"href,omitempty"`
	Tag  string `json:"tag"`
}

type pageSnapshot struct {
	URL       string  `json:"url"`
	Title     string  `json:"title"`
	Headings  []entry `json:"headings"`
	NavLinks  []entry `json:"nav_links"`
	CTA       []entry `json:"cta"`
	BodyHints []entry `json:"body_hints"`
}

func main() {
	var binPath string
	flag.StringVar(&binPath, "bin", "", "浏览器二进制文件路径")
	flag.Parse()

	b := browser.NewBrowser(false, browser.WithBinPath(binPath))
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	page.MustNavigate("https://www.goofish.com/").MustWaitLoad()
	time.Sleep(4 * time.Second)

	jsonText := page.MustEval(`() => {
		const visible = (el) => {
			if (!el) return false;
			const style = window.getComputedStyle(el);
			if (!style) return false;
			if (style.display === 'none' || style.visibility === 'hidden') return false;
			return el.offsetWidth > 0 && el.offsetHeight > 0;
		};

		const collect = (selectors, tag, limit = 40) => {
			const out = [];
			const seen = new Set();
			for (const el of document.querySelectorAll(selectors)) {
				if (!visible(el)) continue;
				const text = (el.textContent || '').replace(/\s+/g, ' ').trim();
				if (!text || text.length > 30) continue;
				const href = el.href || el.getAttribute?.('href') || '';
				const key = text + '::' + href;
				if (seen.has(key)) continue;
				seen.add(key);
				out.push({ text, href, tag });
				if (out.length >= limit) break;
			}
			return out;
		};

		const headings = collect('h1,h2,h3,[class*="title"],[class*="header"]', 'heading', 20);
		const navLinks = collect('header a, nav a, [class*="nav"] a, [class*="menu"] a', 'nav', 50);
		const cta = collect('button,a,[role="button"]', 'cta', 80).filter((x) => {
			return ['发布', '卖', '买', '消息', '我的', '搜索', '客服', '鱼塘', '收藏', '订单'].some((k) => x.text.includes(k));
		});

		const bodyText = (document.body?.innerText || '').replace(/\s+/g, ' ');
		const hintKeywords = ['发布', '我想卖', '我想买', '搜索', '消息', '收藏', '订单', '客服', '鱼塘', '浏览记录', '我的'];
		const bodyHints = hintKeywords
			.filter((k) => bodyText.includes(k))
			.map((k) => ({ text: k, tag: 'hint' }));

		return JSON.stringify({
			url: location.href,
			title: document.title,
			headings,
			nav_links: navLinks,
			cta,
			body_hints: bodyHints,
		});
	}`).String()

	var snap pageSnapshot
	if err := json.Unmarshal([]byte(jsonText), &snap); err != nil {
		logrus.Fatalf("failed to parse snapshot: %v", err)
	}

	fmt.Println("=== XIANYU SNAPSHOT ===")
	prettyPrint(snap)

	fmt.Println("=== CANDIDATE FEATURES ===")
	for _, item := range inferFeatures(snap) {
		fmt.Printf("- %s\n", item)
	}
}

func prettyPrint(s pageSnapshot) {
	buf, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(buf))
}

func inferFeatures(s pageSnapshot) []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	add := func(v string) {
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, v)
	}

	allTexts := make([]string, 0, len(s.CTA)+len(s.NavLinks)+len(s.BodyHints))
	for _, x := range s.CTA {
		allTexts = append(allTexts, x.Text)
	}
	for _, x := range s.NavLinks {
		allTexts = append(allTexts, x.Text)
	}
	for _, x := range s.BodyHints {
		allTexts = append(allTexts, x.Text)
	}
	joined := strings.Join(allTexts, "|")

	if strings.Contains(joined, "发布") || strings.Contains(joined, "卖") {
		add("发布闲置商品（图文发布、价格、分类、标签）")
	}
	if strings.Contains(joined, "搜索") {
		add("搜索商品（关键词、筛选、排序、分页）")
	}
	if strings.Contains(joined, "消息") {
		add("消息会话读取（会话列表、最近消息、未读数）")
	}
	if strings.Contains(joined, "收藏") {
		add("收藏夹读取与管理")
	}
	if strings.Contains(joined, "订单") {
		add("买卖订单查询（状态、发货、确认收货）")
	}
	if strings.Contains(joined, "我的") {
		add("个人主页与在售商品管理")
	}
	if strings.Contains(joined, "鱼塘") {
		add("鱼塘内容浏览与互动")
	}
	if strings.Contains(joined, "客服") {
		add("客服入口与售后记录查询")
	}

	if len(out) == 0 {
		add("主页可见入口较少，建议二次探索：搜索页、商品详情页、消息页、我的主页")
	}
	return out
}
