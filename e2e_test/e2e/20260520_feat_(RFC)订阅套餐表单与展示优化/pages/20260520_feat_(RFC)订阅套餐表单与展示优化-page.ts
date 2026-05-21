import { Page, Locator, expect } from '@playwright/test';

/**
 * 订阅套餐管理页（/subscriptions）+ 钱包页（/wallet）的页面对象
 * 包含：套餐编辑抽屉、套餐卡片展示、购买确认弹窗
 */
export class SubscriptionPlanPage {
  readonly page: Page;

  // ============================================================================
  // 订阅管理页 (/subscriptions)
  // ============================================================================
  readonly createButton: Locator;
  readonly planTable: Locator;
  readonly planRows: Locator;

  // ============================================================================
  // 套餐编辑抽屉
  // ============================================================================
  readonly drawerTitle: Locator;
  readonly titleInput: Locator;
  readonly subtitleInput: Locator;
  readonly planTypeSelect: Locator;
  readonly priceAmountInput: Locator;
  readonly durationUnitSelect: Locator;
  readonly durationValueInput: Locator;
  readonly customSecondsInput: Locator;
  readonly upgradeGroupSelect: Locator;
  readonly purchaseLimitInput: Locator;
  readonly sortOrderInput: Locator;
  readonly enabledSwitch: Locator;
  readonly totalAmountInput: Locator;
  readonly resetPeriodSelect: Locator;
  readonly customResetSecondsInput: Locator;
  readonly tokensPerWindowInput: Locator;
  readonly weeklyMultiplierInput: Locator;
  readonly weeklyLimitPreview: Locator;
  readonly mTokenLabel: Locator;
  readonly saveButton: Locator;
  readonly closeButton: Locator;

  // 抽屉内区域标题
  readonly basicInfoSection: Locator;
  readonly commonFieldsSection: Locator;
  readonly quotaSettingsSection: Locator;
  readonly rateLimitSection: Locator;

  // ============================================================================
  // 钱包页 (/wallet)
  // ============================================================================
  readonly walletHeading: Locator;
  readonly subscriptionSection: Locator;
  readonly apiPlanHeading: Locator;
  readonly codingPlanHeading: Locator;

  // ============================================================================
  // 购买确认弹窗
  // ============================================================================
  readonly purchaseDialogTitle: Locator;
  readonly purchasePlanName: Locator;
  readonly purchaseValidityPeriod: Locator;
  readonly purchaseTotalQuota: Locator;
  readonly purchaseResetPeriod: Locator;
  readonly purchasePer5hAllowance: Locator;
  readonly purchaseWeeklyAllowance: Locator;
  readonly purchaseUpgradeGroup: Locator;
  readonly purchaseAmountDue: Locator;
  readonly purchasePaymentAlert: Locator;
  readonly purchaseCloseButton: Locator;

  // 通知
  readonly successToast: Locator;
  readonly errorToast: Locator;

  constructor(page: Page) {
    this.page = page;

    // 订阅管理页
    this.createButton = page.getByRole('button', { name: '新建套餐' });
    this.planTable = page.getByRole('table');
    this.planRows = page.locator('tbody tr, [role="rowgroup"] > [role="row"]').filter({ has: page.locator('td, [role="cell"]') });

    // 抽屉 - 基本信息区
    this.drawerTitle = page.locator('[role="dialog"], [data-state="open"]').getByRole('heading').first();
    this.titleInput = page.getByLabel(/套餐标题|Plan Title/);
    this.subtitleInput = page.getByLabel(/套餐副标题|Plan Subtitle/);
    this.planTypeSelect = page.locator('[role="dialog"], [data-state="open"]').getByText(/套餐类型|Plan Type/).locator('..').locator('[role="combobox"]').first();

    // 抽屉 - 固定字段区
    const dialog = page.locator('[role="dialog"], [data-state="open"]');
    this.priceAmountInput = page.getByLabel(/实付金额|Actual Amount/);
    this.durationUnitSelect = dialog.getByText(/有效期单位|Duration Unit/).locator('..').locator('[role="combobox"]').first();
    this.durationValueInput = page.getByLabel(/有效期数值|Duration Value/);
    this.customSecondsInput = page.getByLabel(/自定义秒数|Custom Seconds/);
    this.upgradeGroupSelect = dialog.getByText(/升级分组|Upgrade Group/).locator('..').locator('[role="combobox"]').first();
    this.purchaseLimitInput = page.getByLabel(/限购|Purchase Limit/);
    this.sortOrderInput = page.getByLabel(/排序|Sort Order/);
    this.enabledSwitch = page.getByRole('switch', { name: /启用状态|Enabled Status/ });

    // 抽屉 - API 专属字段
    this.totalAmountInput = dialog.getByText(/^总额度$|^Total Quota$/).locator('..').locator('[role="spinbutton"], input[type="number"]').first();
    this.resetPeriodSelect = dialog.getByText(/重置周期|Reset Cycle/).locator('..').locator('[role="combobox"]').first();
    this.customResetSecondsInput = dialog.getByLabel(/自定义秒数|Custom Seconds/).last();

    // 抽屉 - CodingPlan 专属字段
    this.tokensPerWindowInput = dialog.getByText(/5\s*小时.*Token.*上限|5-Hour Token Limit/).locator('..').locator('[role="spinbutton"], input[type="number"]').first();
    this.weeklyMultiplierInput = page.getByLabel(/周倍数|Weekly Multiplier/);
    this.weeklyLimitPreview = page.locator('[role="dialog"], [data-state="open"]').getByText(/周上限:|Weekly limit:/);

    // M token 单位标签（输入框右侧的 span）
    this.mTokenLabel = page.locator('[role="dialog"], [data-state="open"]').getByText('M token');

    // 抽屉 - 按钮
    this.saveButton = page.getByRole('button', { name: /保存更改|Save changes/ });
    this.closeButton = page.getByRole('button', { name: /关闭|Close/ });

    // 抽屉内区域标题
    this.basicInfoSection = page.locator('[role="dialog"], [data-state="open"]').getByText(/基本信息|Basic Info/);
    this.commonFieldsSection = page.locator('[role="dialog"], [data-state="open"]').getByText(/通用字段|Common Fields/);
    this.quotaSettingsSection = page.locator('[role="dialog"], [data-state="open"]').getByText(/额度设置|Quota Settings/);
    this.rateLimitSection = page.locator('[role="dialog"], [data-state="open"]').getByText(/速率限制|Rate Limit/);

    // 钱包页
    this.walletHeading = page.getByRole('heading', { name: /钱包|Wallet/ });
    this.subscriptionSection = page.getByText(/Subscription Plans|订阅套餐/).first();
    this.apiPlanHeading = page.getByRole('heading', { level: 3, name: /API 套餐|API Plan/ });
    this.codingPlanHeading = page.getByRole('heading', { level: 3, name: /CodingPlan 套餐|编程套餐/ });

    // 购买弹窗
    this.purchaseDialogTitle = page.getByRole('dialog').getByRole('heading', { name: /购买订阅|Purchase Subscription/ });
    this.purchasePlanName = page.getByRole('dialog').getByText(/套餐名称|Plan Name/).locator('..').locator('div, span, p').last();
    this.purchaseValidityPeriod = page.getByRole('dialog').getByText(/^有效期$|^Validity Period$/).locator('..').locator('div, span, p').last();
    this.purchaseTotalQuota = page.getByRole('dialog').getByText(/^总额度$|^Total Quota$/).locator('..').locator('div, span, p').last();
    this.purchaseResetPeriod = page.getByRole('dialog').getByText(/重置周期|Reset Period/).locator('..').locator('div, span, p').last();
    this.purchasePer5hAllowance = page.getByRole('dialog').getByText(/每.*5.*小时额度|Per 5h allowance/).locator('..').locator('div, span, p').last();
    this.purchaseWeeklyAllowance = page.getByRole('dialog').getByText(/每周额度|Weekly allowance/).locator('..').locator('div, span, p').last();
    this.purchaseUpgradeGroup = page.getByRole('dialog').getByText(/升级组|Upgrade Group/).locator('..');
    this.purchaseAmountDue = page.getByRole('dialog').getByText(/应付金额|Amount Due/).locator('..').locator('div, span, p').last();
    this.purchasePaymentAlert = page.getByRole('dialog').getByText(/在线支付|Online payment is not enabled/);
    this.purchaseCloseButton = page.getByRole('dialog').getByRole('button', { name: /Close|关闭/ });

    // 通知
    this.successToast = page.getByText(/Create succeeded|创建成功|Update succeeded|更新成功/);
    this.errorToast = page.locator('[data-sonner-toast][data-type="error"]');
  }

  // ============================================================================
  // 导航方法
  // ============================================================================

  async gotoAdmin(): Promise<void> {
    await this.page.goto('/subscriptions');
    await this.page.getByRole('heading', { name: /订阅管理|Subscription/ }).waitFor({ state: 'visible' });
  }

  async gotoWallet(): Promise<void> {
    await this.page.goto('/wallet');
    await this.walletHeading.waitFor({ state: 'visible' });
  }

  // ============================================================================
  // 管理后台操作
  // ============================================================================

  async clickCreate(): Promise<void> {
    await this.createButton.click();
    // 等待抽屉动画完成
    await this.titleInput.waitFor({ state: 'visible' });
  }

  async clickEditById(id: number): Promise<void> {
    const row = this.planTable.locator('tr').filter({ hasText: `#${id}` });
    await row.locator('button').last().click();
    // 等待下拉菜单
    const editItem = this.page.getByRole('menuitem', { name: /编辑|Edit/ });
    await editItem.waitFor({ state: 'visible' });
    await editItem.click();
    await this.titleInput.waitFor({ state: 'visible' });
  }

  async selectPlanType(type: 'api' | 'coding_plan'): Promise<void> {
    await this.planTypeSelect.click();
    const optionText = type === 'api' ? /API/ : /CodingPlan/;
    const option = this.page.getByRole('option').filter({ hasText: optionText });
    await option.waitFor({ state: 'visible' });
    await option.click();
  }

  async selectDurationUnit(unit: string): Promise<void> {
    await this.durationUnitSelect.click();
    const option = this.page.getByRole('option').filter({ hasText: new RegExp(unit, 'i') });
    await option.waitFor({ state: 'visible' });
    await option.click();
  }

  async selectResetPeriod(period: string): Promise<void> {
    await this.resetPeriodSelect.click();
    const option = this.page.getByRole('option').filter({ hasText: new RegExp(period, 'i') });
    await option.waitFor({ state: 'visible' });
    await option.click();
  }

  async selectUpgradeGroup(group: string): Promise<void> {
    await this.upgradeGroupSelect.click();
    if (group === '') {
      const option = this.page.getByRole('option').filter({ hasText: /不升级|No Upgrade/ });
      await option.waitFor({ state: 'visible' });
      await option.click();
    } else {
      const option = this.page.getByRole('option', { name: group });
      await option.waitFor({ state: 'visible' });
      await option.click();
    }
  }

  async save(): Promise<void> {
    await this.saveButton.click();
  }

  // ============================================================================
  // 钱包页操作
  // ============================================================================

  async getApiPlanCards(): Promise<Locator[]> {
    if (!(await this.apiPlanHeading.isVisible())) return [];
    const section = this.apiPlanHeading.locator('..');
    return await section.locator('[class*="card"], [data-slot="card-content"]').all();
  }

  async getCodingPlanCards(): Promise<Locator[]> {
    if (!(await this.codingPlanHeading.isVisible())) return [];
    const section = this.codingPlanHeading.locator('..');
    return await section.locator('[class*="card"], [data-slot="card-content"]').all();
  }

  async clickSubscribe(title: string): Promise<void> {
    // heading 嵌套 3 层：h4 → div.min-w-0 → div.mb-2 → div.card-content (含按钮)
    const cardHeading = this.page.getByRole('heading', { level: 4, name: title });
    const card = cardHeading.locator('../../..');
    await card.getByRole('button', { name: /立即订阅|Subscribe Now/ }).click();
    await this.purchaseDialogTitle.waitFor({ state: 'visible' });
  }

  closePurchaseDialog(): Promise<void> {
    return this.purchaseCloseButton.click();
  }

  // ============================================================================
  // 断言辅助
  // ============================================================================

  async isSectionVisible(sectionLocator: Locator): Promise<boolean> {
    try {
      await sectionLocator.waitFor({ state: 'visible', timeout: 2000 });
      return true;
    } catch {
      return false;
    }
  }

  async getPlanIdByTitle(title: string): Promise<number | null> {
    const row = this.planTable.locator('tr').filter({ hasText: title });
    try {
      await row.waitFor({ state: 'visible', timeout: 3000 });
      const idText = await row.locator('td').first().innerText();
      const match = idText.match(/#(\d+)/);
      return match ? parseInt(match[1], 10) : null;
    } catch {
      return null;
    }
  }
}
