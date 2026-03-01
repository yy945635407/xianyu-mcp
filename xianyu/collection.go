package xianyu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

const collectionURL = "https://www.goofish.com/collection"

type CollectionAction struct {
	page *rod.Page
}

type CollectionGroup struct {
	Name     string `json:"name"`
	Count    int    `json:"count,omitempty"`
	Selected bool   `json:"selected,omitempty"`
	RawText  string `json:"raw_text,omitempty"`
}

type CollectionItem struct {
	Title    string `json:"title"`
	Price    string `json:"price,omitempty"`
	URL      string `json:"url"`
	ItemID   string `json:"item_id,omitempty"`
	Seller   string `json:"seller,omitempty"`
	Group    string `json:"group,omitempty"`
	RawText  string `json:"raw_text,omitempty"`
	FavorTag string `json:"favor_tag,omitempty"`
}

type CollectionListResult struct {
	CurrentGroup string            `json:"current_group,omitempty"`
	Groups       []CollectionGroup `json:"groups"`
	Count        int               `json:"count"`
	Items        []CollectionItem  `json:"items"`
}

type CollectionActionResult struct {
	Success        bool   `json:"success"`
	Operation      string `json:"operation,omitempty"`
	Message        string `json:"message"`
	MatchedItem    string `json:"matched_item,omitempty"`
	TargetGroup    string `json:"target_group,omitempty"`
	RequiresManual bool   `json:"requires_manual,omitempty"`
}

func NewCollectionAction(page *rod.Page) *CollectionAction {
	return &CollectionAction{page: page.Timeout(60 * time.Second)}
}

func (a *CollectionAction) ListCollections(ctx context.Context, group string, limit int) (*CollectionListResult, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(collectionURL).MustWaitLoad()
	if !waitCollectionReady(pp) {
		return nil, fmt.Errorf("collection page not ready")
	}

	if strings.TrimSpace(group) != "" && group != "全部" {
		_ = switchCollectionGroup(pp, group)
		time.Sleep(900 * time.Millisecond)
	}

	extract := func() (string, error) {
		return pp.MustEval(`(limit) => {
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

		const selectedGroup = (() => {
			const selected = document.querySelector('div[class*="textSelected"], div[class*="selected"], span[class*="selected"]');
			return clean(selected?.textContent || '');
		})();

		const groupCandidates = Array.from(document.querySelectorAll('div[class*="title"] div[class*="text"], div[class*="textReal"], div[class*="tab"], span[class*="tab"], a[class*="tab"]'));
		const groups = [];
		const seenGroup = new Set();
		for (const el of groupCandidates) {
			const txt = clean(el.textContent || '');
			if (!txt || txt.length > 20) continue;
			if (!/(全部|降价|有效|失效|分组|收藏)/.test(txt)) continue;
			if (seenGroup.has(txt)) continue;
			seenGroup.add(txt);
			const cnt = parseInt((txt.match(/\d+/) || [])[0] || '0', 10) || 0;
			groups.push({
				name: txt,
				count: cnt,
				selected: selectedGroup && txt.includes(selectedGroup),
				raw_text: txt,
			});
		}

		const cards = Array.from(document.querySelectorAll('a[href*="/item?id="], a[href*="itemId="]'));
		const out = [];
		const seen = new Set();
		for (const a of cards) {
			const href = toAbsURL(a.getAttribute('href') || a.href || '');
			if (!href) continue;
			const id = parseID(href);
			const card = a.closest('div[class*="feeds-item-wrap"], li[class*="item"], article') || a.parentElement || a;
			const anchorText = clean(a.innerText || '');
			let rawText = clean(card.innerText || anchorText || '');
			if (rawText.length > 220 && anchorText) {
				rawText = anchorText;
			}
			if (!rawText) continue;

			let title = rawText
				.replace(/取消收藏/g, '')
				.replace(/收藏后\s*¥?\s*\d+(?:\.\d+)?/g, '')
				.replace(/¥\s*\d+(?:\.\d+)?/g, ' ')
				.replace(/\s+/g, ' ')
				.trim();
			if (!title || title.length > 100) {
				title = clean(a.getAttribute('title') || a.textContent || rawText).slice(0, 80);
			}

			const priceMatch = rawText.match(/¥\s*[0-9]+(?:\.[0-9]+)?/);
			const price = priceMatch ? clean(priceMatch[0]) : '';
			const favorTag = clean(card.querySelector('[class*="reducePrice"], [class*="favorText"]')?.textContent || '');

			const key = String(id || '') + '::' + String(href || '');
			if (seen.has(key)) continue;
			seen.add(key);
			out.push({
				title,
				price,
				url: href,
				item_id: id,
				group: selectedGroup,
				raw_text: rawText.slice(0, 180),
				favor_tag: favorTag,
			});
			if (out.length >= limit) break;
		}

		return JSON.stringify({
			current_group: selectedGroup,
			groups,
			count: out.length,
			items: out,
		});
	}`, limit).String(), nil
	}

	var result CollectionListResult
	var lastErr error
	for i := 0; i < 4; i++ {
		raw, err := extract()
		if err != nil {
			lastErr = err
			time.Sleep(700 * time.Millisecond)
			continue
		}
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			lastErr = fmt.Errorf("unmarshal collection list failed: %w", err)
			time.Sleep(700 * time.Millisecond)
			continue
		}
		if result.Count > 0 || (len(result.Groups) > 0 && i >= 1) {
			break
		}
		time.Sleep(900 * time.Millisecond)
	}
	if lastErr != nil && result.Count == 0 && len(result.Groups) == 0 {
		return nil, lastErr
	}

	result.CurrentGroup = normalizeCollectionGroupName(result.CurrentGroup)
	for i := range result.Groups {
		result.Groups[i].Name = normalizeCollectionGroupName(result.Groups[i].Name)
		result.Groups[i].RawText = normalizeCollectionGroupName(result.Groups[i].RawText)
	}
	return &result, nil
}

func (a *CollectionAction) CancelFavorite(ctx context.Context, keyword, itemRef string) (*CollectionActionResult, error) {
	kw := strings.TrimSpace(keyword)
	ref := strings.TrimSpace(itemRef)
	if kw == "" && ref == "" {
		return nil, fmt.Errorf("keyword or item_ref is required")
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(collectionURL).MustWaitLoad()
	if !waitCollectionReady(pp) {
		return nil, fmt.Errorf("collection page not ready")
	}

	raw := pp.MustEval(`(keyword, itemRef) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const normalize = (s) => clean(s).toLowerCase().replace(/[^0-9a-z\u4e00-\u9fa5]/g, '');
		const parseID = (href) => {
			try {
				const u = new URL(href, location.origin);
				return u.searchParams.get('id') || u.searchParams.get('itemId') || '';
			} catch {
				const m = (href || '').match(/(?:id|itemId)=([0-9]+)/);
				return m ? m[1] : '';
			}
		};

		const normKW = normalize(keyword);
		const normRef = normalize(itemRef);
		const cards = Array.from(document.querySelectorAll('a[href*="/item?id="], a[href*="itemId="]'));
		for (const a of cards) {
			const href = (a.getAttribute('href') || a.href || '');
			const id = parseID(href);
			const card = a.closest('div[class*="feeds-item-wrap"], div[class*="item"], li, article, div') || a;
			const txt = clean(card.innerText || a.innerText || '');
			const normTxt = normalize(txt);
			const normHref = normalize(href);
			const normID = normalize(id);
			const matched =
				(normKW && normTxt.includes(normKW)) ||
				(normRef && (normHref.includes(normRef) || normID === normRef || normTxt.includes(normRef)));
			if (!matched) continue;

			const ctrls = Array.from(card.querySelectorAll('button,div,span,a'));
			const cancelBtn = ctrls.find((el) => clean(el.textContent || '') === '取消收藏' || clean(el.textContent || '').includes('取消收藏'));
			if (!cancelBtn) {
				return JSON.stringify({ ok: false, message: '命中商品但未找到取消收藏按钮', matched: txt.slice(0, 80) });
			}
			cancelBtn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
			cancelBtn.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
			cancelBtn.click();
			return JSON.stringify({ ok: true, message: '已触发取消收藏', matched: txt.slice(0, 80), id });
		}

		return JSON.stringify({ ok: false, message: '未找到匹配收藏商品', matched: '' });
	}`, kw, ref).String()

	type cancelResp struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
		Matched string `json:"matched"`
	}
	var resp cancelResp
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("parse cancel favorite result failed: %w", err)
	}

	return &CollectionActionResult{
		Success:     resp.OK,
		Operation:   "cancel_favorite",
		Message:     resp.Message,
		MatchedItem: resp.Matched,
	}, nil
}

func (a *CollectionAction) ManageGroup(ctx context.Context, operation, groupName, newName, itemKeyword string) (*CollectionActionResult, error) {
	op := strings.ToLower(strings.TrimSpace(operation))
	if op == "" {
		return nil, fmt.Errorf("operation is required")
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(collectionURL).MustWaitLoad()
	if !waitCollectionReady(pp) {
		return nil, fmt.Errorf("collection page not ready")
	}

	raw := pp.MustEval(`(op, groupName, newName, itemKeyword) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const normalize = (s) => clean(s).toLowerCase().replace(/[^0-9a-z\u4e00-\u9fa5]/g, '');
		const normGroup = normalize(groupName);
		const normNew = clean(newName);
		const normKeyword = normalize(itemKeyword);

		const clickByText = (patterns) => {
			const nodes = Array.from(document.querySelectorAll('button,div,span,a'));
			for (const el of nodes) {
				const txt = clean(el.textContent || '');
				if (!txt || txt.length > 20) continue;
				if (!patterns.some((p) => txt === p || txt.includes(p))) continue;
				const visible = !!(el.offsetWidth || el.offsetHeight || el.getClientRects().length);
				if (!visible) continue;
				el.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
				el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
				el.click();
				return txt;
			}
			return '';
		};

		const setInput = (val) => {
			const input = document.querySelector('input[placeholder*="分组"], input[placeholder*="名称"], input.ant-input, textarea');
			if (!input) return false;
			input.focus();
			const proto = input.tagName.toLowerCase() === 'textarea'
				? window.HTMLTextAreaElement.prototype
				: window.HTMLInputElement.prototype;
			const setter = Object.getOwnPropertyDescriptor(proto, 'value')?.set;
			if (setter) {
				setter.call(input, '');
				input.dispatchEvent(new Event('input', { bubbles: true }));
				setter.call(input, val);
				input.dispatchEvent(new Event('input', { bubbles: true }));
			} else {
				input.value = val;
				input.dispatchEvent(new Event('input', { bubbles: true }));
			}
			input.dispatchEvent(new Event('change', { bubbles: true }));
			return true;
		};

		const openManage = () => {
			const txt = clickByText(['分组管理', '管理分组', '管理']);
			return !!txt;
		};

		if (op === 'create') {
			openManage();
			clickByText(['新建分组', '创建分组', '新建']);
			if (!normNew && !clean(groupName)) {
				return JSON.stringify({ ok: false, message: 'create 需要 group_name/new_name 参数', requires_manual: false });
			}
			const val = normNew || clean(groupName);
			if (!setInput(val)) {
				return JSON.stringify({ ok: false, message: '未找到分组输入框，需手动创建分组', requires_manual: true });
			}
			clickByText(['确定', '确认', '保存', '完成']);
			return JSON.stringify({ ok: true, message: '已尝试创建分组', target_group: val });
		}

		if (op === 'rename') {
			if (!normGroup || !normNew) {
				return JSON.stringify({ ok: false, message: 'rename 需要 group_name + new_name', requires_manual: false });
			}
			openManage();
			const groupNode = Array.from(document.querySelectorAll('div,span,a,li'))
				.find((el) => normalize(el.textContent || '') === normGroup || normalize(el.textContent || '').includes(normGroup));
			if (!groupNode) {
				return JSON.stringify({ ok: false, message: '未找到目标分组', target_group: groupName });
			}
			groupNode.click();
			clickByText(['重命名', '编辑名称', '编辑']);
			if (!setInput(normNew)) {
				return JSON.stringify({ ok: false, message: '未找到重命名输入框，需手动处理', target_group: groupName, requires_manual: true });
			}
			clickByText(['确定', '确认', '保存', '完成']);
			return JSON.stringify({ ok: true, message: '已尝试重命名分组', target_group: normNew });
		}

		if (op === 'delete') {
			if (!normGroup) {
				return JSON.stringify({ ok: false, message: 'delete 需要 group_name', requires_manual: false });
			}
			openManage();
			const groupNode = Array.from(document.querySelectorAll('div,span,a,li'))
				.find((el) => normalize(el.textContent || '') === normGroup || normalize(el.textContent || '').includes(normGroup));
			if (!groupNode) {
				return JSON.stringify({ ok: false, message: '未找到目标分组', target_group: groupName });
			}
			groupNode.click();
			const deleteTxt = clickByText(['删除分组', '删除']);
			if (!deleteTxt) {
				return JSON.stringify({ ok: false, message: '未找到删除分组按钮，需手动处理', target_group: groupName, requires_manual: true });
			}
			clickByText(['确定', '确认']);
			return JSON.stringify({ ok: true, message: '已尝试删除分组', target_group: groupName });
		}

		if (op === 'move') {
			if (!normKeyword || !clean(groupName)) {
				return JSON.stringify({ ok: false, message: 'move 需要 item_keyword + group_name', requires_manual: false });
			}
			const cards = Array.from(document.querySelectorAll('a[href*="/item?id="], a[href*="itemId="]'));
			for (const a of cards) {
				const card = a.closest('div[class*="feeds-item-wrap"], div[class*="item"], li, article, div') || a;
				const txt = clean(card.innerText || a.innerText || '');
				if (!normalize(txt).includes(normKeyword)) continue;
				const controls = Array.from(card.querySelectorAll('button,div,span,a'));
				const moveBtn = controls.find((el) => {
					const t = clean(el.textContent || '');
					return t.includes('分组') || t.includes('移入') || t.includes('移动');
				});
				if (!moveBtn) {
					return JSON.stringify({ ok: false, message: '命中商品但未找到移入分组入口，需手动处理', matched: txt.slice(0, 80), requires_manual: true });
				}
				moveBtn.click();
				clickByText([groupName]);
				clickByText(['确定', '确认', '完成']);
				return JSON.stringify({ ok: true, message: '已尝试移动商品到分组', matched: txt.slice(0, 80), target_group: groupName });
			}
			return JSON.stringify({ ok: false, message: '未找到待移动商品', target_group: groupName });
		}

		return JSON.stringify({ ok: false, message: 'unsupported operation: ' + op, requires_manual: false });
	}`, op, strings.TrimSpace(groupName), strings.TrimSpace(newName), strings.TrimSpace(itemKeyword)).String()

	type groupResp struct {
		OK             bool   `json:"ok"`
		Message        string `json:"message"`
		Matched        string `json:"matched"`
		TargetGroup    string `json:"target_group"`
		RequiresManual bool   `json:"requires_manual"`
	}
	var resp groupResp
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("parse manage group result failed: %w", err)
	}

	if resp.TargetGroup == "" {
		resp.TargetGroup = strings.TrimSpace(groupName)
	}

	return &CollectionActionResult{
		Success:        resp.OK,
		Operation:      op,
		Message:        resp.Message,
		MatchedItem:    resp.Matched,
		TargetGroup:    resp.TargetGroup,
		RequiresManual: resp.RequiresManual,
	}, nil
}

func waitCollectionReady(pp *rod.Page) bool {
	for i := 0; i < 20; i++ {
		ready := pp.MustEval(`() => {
			const items = document.querySelectorAll('a[href*="/item?id="], a[href*="itemId="]').length;
			if (items > 0) return true;
			const txt = (document.body?.innerText || '').replace(/\s+/g, ' ');
			return txt.includes('暂无收藏') || txt.includes('我的收藏');
		}`).Bool()
		if ready {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func switchCollectionGroup(pp *rod.Page, group string) bool {
	target := strings.TrimSpace(group)
	if target == "" {
		return false
	}

	return pp.MustEval(`(group) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const normalize = (s) => clean(s).toLowerCase().replace(/[^0-9a-z\u4e00-\u9fa5]/g, '');
		const target = normalize(group);
		const nodes = Array.from(document.querySelectorAll('div,span,a,button'));
		for (const el of nodes) {
			const txt = clean(el.textContent || '');
			if (!txt || txt.length > 20) continue;
			const norm = normalize(txt);
			if (!(norm === target || norm.includes(target) || target.includes(norm))) continue;
			if (!/(全部|降价|有效|失效|分组|收藏)/.test(txt)) continue;
			const visible = !!(el.offsetWidth || el.offsetHeight || el.getClientRects().length);
			if (!visible) continue;
			el.click();
			return true;
		}
		return false;
	}`, target).Bool()
}

func normalizeCollectionGroupName(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return ""
	}
	runes := []rune(clean)
	if len(runes)%2 == 0 {
		half := len(runes) / 2
		first := strings.TrimSpace(string(runes[:half]))
		second := strings.TrimSpace(string(runes[half:]))
		if first != "" && first == second {
			return first
		}
	}
	return clean
}
