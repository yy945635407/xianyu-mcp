package xianyu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (a *OrderAction) ShipWithLogistics(ctx context.Context, username, company, trackingNo string) (*OrderActionResult, error) {
	user := strings.TrimSpace(username)
	if user == "" {
		return nil, fmt.Errorf("username is required")
	}
	if strings.TrimSpace(trackingNo) == "" {
		return nil, fmt.Errorf("tracking_no is required")
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(imURL).MustWaitLoad()
	time.Sleep(1500 * time.Millisecond)
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
		const nodes = Array.from(document.querySelectorAll('button,div,span,a'));
		const btn = nodes.find((el) => clean(el.textContent || '') === '去发货' || clean(el.textContent || '').includes('去发货'));
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
		return txt.includes('功能待上线') || txt.includes('扫码去APP查看') || txt.includes('去APP查看') || txt.includes('请在闲鱼APP') ;
	}`).Bool()
	if needApp {
		return &OrderActionResult{
			Success:      false,
			RequiresApp:  true,
			Message:      "网页端物流发货能力受限，请转闲鱼 APP 填写物流并确认发货",
			MatchedOrder: clickedName,
		}, nil
	}

	raw := pp.MustEval(`(company, trackingNo) => {
		const setInput = (el, val) => {
			if (!el) return false;
			el.focus();
			const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')?.set;
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
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();

		const companyInput = document.querySelector('input[placeholder*="快递公司"], input[placeholder*="物流公司"], input[name*="company"]');
		const trackingInput = document.querySelector('input[placeholder*="运单号"], input[placeholder*="快递单号"], input[placeholder*="物流单号"], input[name*="tracking"], input[name*="waybill"]');

		const companyFilled = setInput(companyInput, company || '');
		const trackingFilled = setInput(trackingInput, trackingNo || '');

		const submit = Array.from(document.querySelectorAll('button,div,span,a')).find((el) => {
			const t = clean(el.textContent || '');
			return t === '确认发货' || t === '提交' || t === '完成' || t.includes('确认发货');
		});
		if (submit) submit.click();

		return JSON.stringify({
			company_filled: companyFilled,
			tracking_filled: trackingFilled,
			submit_clicked: !!submit,
		});
	}`, company, trackingNo).String()

	type shipResp struct {
		CompanyFilled  bool `json:"company_filled"`
		TrackingFilled bool `json:"tracking_filled"`
		SubmitClicked  bool `json:"submit_clicked"`
	}
	var resp shipResp
	_ = json.Unmarshal([]byte(raw), &resp)
	if !resp.TrackingFilled {
		return &OrderActionResult{
			Success:      false,
			Message:      "未找到网页端物流输入框，请转 APP 发货",
			RequiresApp:  true,
			MatchedOrder: clickedName,
		}, nil
	}

	return &OrderActionResult{
		Success:      true,
		Message:      "已尝试填写物流并提交发货",
		MatchedOrder: clickedName,
	}, nil
}

func (a *OrderAction) ConfirmReceipt(ctx context.Context, orderKeyword, sellerName string) (*OrderActionResult, error) {
	pp := a.page.Context(ctx)
	pp.MustNavigate(boughtURL).MustWaitLoad()
	time.Sleep(1200 * time.Millisecond)
	_ = switchOrderTab(pp, "待收货")
	time.Sleep(1000 * time.Millisecond)

	raw := pp.MustEval(`(orderKeyword, sellerName) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const kw = clean(orderKeyword);
		const seller = clean(sellerName);
		const cards = Array.from(document.querySelectorAll('div[class*="container--Bhfvcld8"]'));

		let target = null;
		for (const card of cards) {
			const txt = clean(card.innerText || '');
			const okKW = !kw || txt.includes(kw);
			const okSeller = !seller || txt.includes(seller);
			if (okKW && okSeller) {
				target = card;
				break;
			}
		}
		if (!target) {
			return JSON.stringify({ ok: false, msg: '未找到匹配待收货订单', matched: '' });
		}

		const matched = clean(target.querySelector('div[class*="desc--"]')?.textContent || target.innerText).slice(0, 80);
		const btn = Array.from(target.querySelectorAll('button,div,span,a')).find((el) => {
			const t = clean(el.textContent || '');
			return t === '确认收货' || t.includes('确认收货');
		});
		if (!btn) return JSON.stringify({ ok: false, msg: '当前订单无确认收货按钮', matched });
		btn.click();

		setTimeout(() => {
			const nodes = Array.from(document.querySelectorAll('button,div,span,a'));
			const confirm = nodes.find((el) => {
				const t = clean(el.textContent || '');
				return t === '确认收货' || t === '确定' || t.includes('确认');
			});
			if (confirm) confirm.click();
		}, 30);

		return JSON.stringify({ ok: true, msg: '已触发确认收货', matched });
	}`, orderKeyword, sellerName).String()

	type confirmResp struct {
		OK      bool   `json:"ok"`
		Msg     string `json:"msg"`
		Matched string `json:"matched"`
	}
	var resp confirmResp
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("parse confirm receipt result failed: %w", err)
	}

	return &OrderActionResult{Success: resp.OK, Message: resp.Msg, MatchedOrder: resp.Matched}, nil
}

func (a *OrderAction) ReviewOrder(ctx context.Context, orderKeyword, sellerName string, score int, content string) (*OrderActionResult, error) {
	if score <= 0 || score > 5 {
		score = 5
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(boughtURL).MustWaitLoad()
	time.Sleep(1200 * time.Millisecond)
	_ = switchOrderTab(pp, "待评价")
	time.Sleep(1000 * time.Millisecond)

	raw := pp.MustEval(`(orderKeyword, sellerName, score, content) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const kw = clean(orderKeyword);
		const seller = clean(sellerName);
		const cards = Array.from(document.querySelectorAll('div[class*="container--Bhfvcld8"]'));

		let target = null;
		for (const card of cards) {
			const txt = clean(card.innerText || '');
			const okKW = !kw || txt.includes(kw);
			const okSeller = !seller || txt.includes(seller);
			if (okKW && okSeller) {
				target = card;
				break;
			}
		}
		if (!target) return JSON.stringify({ ok: false, msg: '未找到匹配待评价订单', matched: '' });

		const matched = clean(target.querySelector('div[class*="desc--"]')?.textContent || target.innerText).slice(0, 80);
		const reviewBtn = Array.from(target.querySelectorAll('button,div,span,a')).find((el) => {
			const t = clean(el.textContent || '');
			return t === '去评价' || t.includes('评价');
		});
		if (!reviewBtn) return JSON.stringify({ ok: false, msg: '当前订单无评价入口', matched });
		reviewBtn.click();

		setTimeout(() => {
			const stars = Array.from(document.querySelectorAll('[class*="star"], i, span')).filter((el) => {
				const cls = (el.className || '').toString();
				return cls.includes('star');
			});
			if (stars.length >= score) {
				stars[score - 1].click();
			}

			const input = document.querySelector('textarea[placeholder*="评价"], textarea[placeholder*="说"]');
			if (input && content) {
				const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value')?.set;
				if (setter) {
					setter.call(input, content);
					input.dispatchEvent(new Event('input', { bubbles: true }));
				} else {
					input.value = content;
					input.dispatchEvent(new Event('input', { bubbles: true }));
				}
				input.dispatchEvent(new Event('change', { bubbles: true }));
			}

			const submit = Array.from(document.querySelectorAll('button,div,span,a')).find((el) => {
				const t = clean(el.textContent || '');
				return t === '提交评价' || t === '发布评价' || t === '完成' || t.includes('提交');
			});
			if (submit) submit.click();
		}, 60);

		return JSON.stringify({ ok: true, msg: '已触发评价流程', matched });
	}`, orderKeyword, sellerName, score, strings.TrimSpace(content)).String()

	type reviewResp struct {
		OK      bool   `json:"ok"`
		Msg     string `json:"msg"`
		Matched string `json:"matched"`
	}
	var resp reviewResp
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("parse review result failed: %w", err)
	}

	return &OrderActionResult{Success: resp.OK, Message: resp.Msg, MatchedOrder: resp.Matched}, nil
}

func (a *OrderAction) HandleRefund(ctx context.Context, orderKeyword, sellerName, action, reason string) (*OrderActionResult, error) {
	mode := strings.TrimSpace(action)
	if mode == "" {
		mode = "detail"
	}

	pp := a.page.Context(ctx)
	pp.MustNavigate(boughtURL).MustWaitLoad()
	time.Sleep(1000 * time.Millisecond)
	_ = switchOrderTab(pp, "退款中")
	time.Sleep(1200 * time.Millisecond)

	raw := pp.MustEval(`(orderKeyword, sellerName, action, reason) => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const kw = clean(orderKeyword);
		const seller = clean(sellerName);
		const mode = clean(action).toLowerCase();
		const cards = Array.from(document.querySelectorAll('div[class*="container--Bhfvcld8"]'));

		let target = null;
		for (const card of cards) {
			const txt = clean(card.innerText || '');
			const okKW = !kw || txt.includes(kw);
			const okSeller = !seller || txt.includes(seller);
			if (okKW && okSeller) {
				target = card;
				break;
			}
		}
		if (!target) return JSON.stringify({ ok: false, msg: '未找到匹配退款订单', matched: '' });

		const matched = clean(target.querySelector('div[class*="desc--"]')?.textContent || target.innerText).slice(0, 80);
		const controls = Array.from(target.querySelectorAll('button,div,span,a'));
		const pickByText = (keys) => controls.find((el) => {
			const t = clean(el.textContent || '');
			return keys.some((k) => t.includes(k));
		});

		let keys = ['退款详情'];
		if (mode.includes('contact')) keys = ['联系卖家'];
		if (mode.includes('complaint')) keys = ['投诉卖家'];
		if (mode.includes('money')) keys = ['查看钱款'];
		if (mode.includes('snapshot')) keys = ['宝贝快照'];
		if (mode.includes('delete')) keys = ['删除订单'];

		const btn = pickByText(keys);
		if (!btn) {
			return JSON.stringify({ ok: false, msg: '当前退款订单缺少目标操作按钮', matched, requires_manual: true });
		}
		btn.click();
		return JSON.stringify({ ok: true, msg: '已触发退款处理动作', matched });
	}`, orderKeyword, sellerName, mode, reason).String()

	type refundResp struct {
		OK             bool   `json:"ok"`
		Msg            string `json:"msg"`
		Matched        string `json:"matched"`
		RequiresManual bool   `json:"requires_manual"`
	}
	var resp refundResp
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, fmt.Errorf("parse refund action result failed: %w", err)
	}

	return &OrderActionResult{
		Success:      resp.OK,
		Message:      resp.Msg,
		MatchedOrder: resp.Matched,
		RequiresApp:  false,
	}, nil
}
