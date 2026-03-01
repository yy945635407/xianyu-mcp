package xianyu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

const accountURL = "https://www.goofish.com/account"

type AccountAction struct {
	page *rod.Page
}

type AccountField struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

type AccountSecurityInfo struct {
	Nickname        string         `json:"nickname,omitempty"`
	MemberName      string         `json:"member_name,omitempty"`
	BasicSettings   []AccountField `json:"basic_settings,omitempty"`
	Certifications  []AccountField `json:"certifications,omitempty"`
	SecurityCenter  AccountField   `json:"security_center,omitempty"`
	SecurityURL     string         `json:"security_url,omitempty"`
	HasRealPerson   bool           `json:"has_real_person,omitempty"`
	HasAlipayVerify bool           `json:"has_alipay_verify,omitempty"`
	RawText         string         `json:"raw_text,omitempty"`
}

func NewAccountAction(page *rod.Page) *AccountAction {
	return &AccountAction{page: page.Timeout(60 * time.Second)}
}

func (a *AccountAction) GetAccountSecurity(ctx context.Context) (*AccountSecurityInfo, error) {
	pp := a.page.Context(ctx)
	pp.MustNavigate(accountURL).MustWaitLoad()
	if !waitAccountPageReady(pp) {
		return nil, fmt.Errorf("account page not ready")
	}

	// ensure left nav switched to 账号与安全
	_ = pp.MustEval(`() => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const nodes = Array.from(document.querySelectorAll('button,div,span,a'));
		const target = nodes.find((el) => clean(el.textContent || '') === '账号与安全' || clean(el.textContent || '').includes('账号与安全'));
		if (target) target.click();
	}`)
	time.Sleep(600 * time.Millisecond)

	raw := pp.MustEval(`() => {
		const clean = (s) => (s || '').replace(/\s+/g, ' ').trim();
		const body = clean(document.body?.innerText || '');

		const accountBlock = document.querySelector('div[class*="accountItem"], div[class*="info"]') || document.body;
		const memberName = clean((body.match(/会员名\s*([a-zA-Z0-9_\-]+)/) || [])[1] || '');
		const nickname = clean(document.querySelector('div[class*="nick"], div[class*="name"]')?.textContent || '');

		const rows = Array.from(document.querySelectorAll('div[class*="row"], li, div[class*="item"]'))
			.map((row) => {
				const c1 = clean(row.querySelector('div[class*="col1"],span[class*="col1"]')?.textContent || '');
				const c2 = clean(row.querySelector('div[class*="col2"],span[class*="col2"]')?.textContent || '');
				const c3 = clean(row.querySelector('div[class*="col3"],span[class*="col3"]')?.textContent || '');
				const txt = clean(row.textContent || '');
				if (!c1 && !txt) return null;
				const name = c1 || txt.slice(0, 20);
				const desc = c2 || '';
				const status = c3 || (txt.includes('已认证') ? '已认证' : (txt.includes('已上传') ? '已上传' : ''));
				return { name, description: desc, status, raw: txt };
			})
			.filter(Boolean)
			.filter((r) => r.name && r.name.length <= 40)
			.filter((r) => !/统一社会信用代码|许可证|备案/.test(r.raw));

		const basicSettings = rows.filter((r) => /保持登录|接收手机通知|会员名|登录|通知/.test(r.name + r.raw)).map((r) => ({
			name: r.name,
			description: r.description,
			status: r.status,
		}));

		const certs = rows.filter((r) => /认证|身份|支付宝|实名/.test(r.name + r.raw)).map((r) => ({
			name: r.name,
			description: r.description,
			status: r.status,
		}));

		const secRow = rows.find((r) => /安全中心/.test(r.name + r.raw)) || { name: '安全中心', description: '', status: '' };
		const securityLink = Array.from(document.querySelectorAll('a[href]')).find((a) => /security|safe/.test(a.getAttribute('href') || '') || clean(a.textContent || '') === '查看');

		return JSON.stringify({
			nickname,
			member_name: memberName,
			basic_settings: basicSettings,
			certifications: certs,
			security_center: {
				name: secRow.name || '安全中心',
				description: secRow.description || '',
				status: secRow.status || '',
			},
			security_url: securityLink ? (securityLink.getAttribute('href') || securityLink.href || '') : '',
			has_real_person: certs.some((x) => (x.name + x.description).includes('实人认证') && (x.status || '').includes('已认证')),
			has_alipay_verify: certs.some((x) => (x.name + x.description).includes('支付宝') && (x.status || '').includes('已认证')),
			raw_text: body.slice(0, 800),
		});
	}`).String()

	var info AccountSecurityInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, fmt.Errorf("unmarshal account security info failed: %w", err)
	}
	info.SecurityURL = strings.TrimSpace(info.SecurityURL)
	return &info, nil
}

func waitAccountPageReady(pp *rod.Page) bool {
	for i := 0; i < 20; i++ {
		ready := pp.MustEval(`() => {
			const txt = (document.body?.innerText || '').replace(/\s+/g, ' ');
			return txt.includes('账号与安全') && (txt.includes('基本信息') || txt.includes('认证信息') || txt.includes('安全中心'));
		}`).Bool()
		if ready {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
