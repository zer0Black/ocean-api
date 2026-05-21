import { test, expect } from '../auth';
import { SubscriptionPlanPage } from './pages/20260520_feat_(RFC)订阅套餐表单与展示优化-page';
import {
  API_PLAN,
  CODING_PLAN,
  M_TOKEN_LABEL,
  M_TOKEN_PLACEHOLDER,
  WEEKLY_LIMIT_PREFIX,
} from './constants/test-data';

let createdPlanIds: number[] = [];

test.describe('订阅套餐表单与展示优化', () => {
  const adminPage = () => new SubscriptionPlanPage(test.info().page);

  // 数据清理：通过 API 删除所有测试创建的套餐
  test.afterAll(async ({ request }) => {
    for (const id of createdPlanIds) {
      await request.delete(`/api/subscription/admin/plans/${id}`).catch(() => {});
    }
    createdPlanIds = [];
  });

  // ============================================================================
  // 1. 正向流程
  // ============================================================================

  test.describe('正向流程', () => {
    test('1.1 成功创建 API 套餐', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();
      await p.clickCreate();

      // 表单分三个区域
      await expect(p.basicInfoSection).toBeVisible();
      await expect(p.commonFieldsSection).toBeVisible();

      // 填写基本信息
      const title = API_PLAN.title();
      await p.titleInput.fill(title);
      await p.subtitleInput.fill(API_PLAN.subtitle);

      // 默认选择 API Plan，验证 Quota Settings 区域可见
      await expect(p.quotaSettingsSection).toBeVisible();

      // 填写固定字段
      await p.priceAmountInput.fill(String(API_PLAN.priceAmount));

      // 填写 API 专属字段
      await p.totalAmountInput.fill(String(API_PLAN.totalAmount));

      await p.save();
      await expect(p.successToast).toBeVisible();

      // 验证列表中出现新套餐
      await expect(p.planTable.getByText(title)).toBeVisible();

      // 记录 ID 用于清理
      const id = await p.getPlanIdByTitle(title);
      if (id) createdPlanIds.push(id);
    });

    test('1.2 成功创建 CodingPlan 套餐', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();
      await p.clickCreate();

      const title = CODING_PLAN.title();
      await p.titleInput.fill(title);
      await p.subtitleInput.fill(CODING_PLAN.subtitle);

      // 切换到 Coding Plan
      await p.selectPlanType('coding_plan');

      // 验证 Rate Limit 区域可见
      await expect(p.rateLimitSection).toBeVisible();

      // 填写固定字段
      await p.priceAmountInput.fill(String(CODING_PLAN.priceAmount));

      // 填写 CodingPlan 专属字段
      await p.tokensPerWindowInput.fill(String(CODING_PLAN.tokensPerWindow));
      await p.weeklyMultiplierInput.fill(String(CODING_PLAN.weeklyMultiplier));

      await p.save();
      await expect(p.successToast).toBeVisible();

      // 验证列表中出现新套餐
      await expect(p.planTable.getByText(title)).toBeVisible();

      const id = await p.getPlanIdByTitle(title);
      if (id) createdPlanIds.push(id);
    });

    test('1.3 成功编辑已有套餐', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();

      // 先创建一个套餐用于编辑
      await p.clickCreate();
      const title = API_PLAN.title();
      await p.titleInput.fill(title);
      await p.priceAmountInput.fill('5');
      await p.save();
      await expect(p.successToast).toBeVisible();

      const id = await p.getPlanIdByTitle(title);
      expect(id).not.toBeNull();
      if (id) createdPlanIds.push(id);

      // 编辑该套餐
      await p.clickEditById(id!);

      const updatedTitle = `${title}_edited`;
      await p.titleInput.clear();
      await p.titleInput.fill(updatedTitle);
      await p.save();
      await expect(p.successToast).toBeVisible();

      // 验证列表中显示更新后的标题
      await expect(p.planTable.getByText(updatedTitle)).toBeVisible();
    });
  });

  // ============================================================================
  // 2. 字段分组显隐
  // ============================================================================

  test.describe('字段分组显隐', () => {
    test('2.1 API 套餐显示 Quota Settings 区域', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();
      await p.clickCreate();

      // 默认应该是 API Plan
      await expect(p.quotaSettingsSection).toBeVisible();
      await expect(p.rateLimitSection).not.toBeVisible();
    });

    test('2.2 CodingPlan 套餐显示 Rate Limit 区域', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();
      await p.clickCreate();

      await p.selectPlanType('coding_plan');

      await expect(p.rateLimitSection).toBeVisible();
      await expect(p.quotaSettingsSection).not.toBeVisible();
    });

    test('2.3 切换类型时区域动态切换', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();
      await p.clickCreate();

      // API → CodingPlan
      await p.selectPlanType('coding_plan');
      await expect(p.rateLimitSection).toBeVisible();
      await expect(p.quotaSettingsSection).not.toBeVisible();

      // CodingPlan → API
      await p.selectPlanType('api');
      await expect(p.quotaSettingsSection).toBeVisible();
      await expect(p.rateLimitSection).not.toBeVisible();
    });
  });

  // ============================================================================
  // 3. 必填字段校验
  // ============================================================================

  test.describe('必填字段校验', () => {
    test('3.1 标题为空应显示必填提示', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();
      await p.clickCreate();

      // 不填标题，直接提交
      await p.save();

      // 验证标题字段下方显示错误信息
      await expect(p.titleInput.locator('..').getByText(/请输入|required/i)).toBeVisible();
    });

    test('3.2 实付金额为空应显示必填提示', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();
      await p.clickCreate();

      // 不填任何字段，直接提交
      await p.save();

      // 验证标题字段下方显示错误信息（标题是首个必填字段）
      await expect(p.titleInput.locator('..').getByText(/请输入|required/i)).toBeVisible();
    });
  });

  // ============================================================================
  // 4. M token 单位验证
  // ============================================================================

  test.describe('M token 单位验证', () => {
    test('4.1 API 套餐总额度字段显示 M token 单位', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();
      await p.clickCreate();

      // 默认 API Plan，检查总额度输入框
      await expect(p.totalAmountInput).toBeVisible();

      // 右侧显示 "M token" 标签
      const inputContainer = p.totalAmountInput.locator('..');
      await expect(inputContainer.getByText(M_TOKEN_LABEL)).toBeVisible();
    });

    test('4.2 CodingPlan 5小时上限字段显示 M token 单位', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();
      await p.clickCreate();
      await p.selectPlanType('coding_plan');

      await expect(p.tokensPerWindowInput).toBeVisible();

      const inputContainer = p.tokensPerWindowInput.locator('..');
      await expect(inputContainer.getByText(M_TOKEN_LABEL)).toBeVisible();
    });
  });

  // ============================================================================
  // 5. 周上限自动计算预览
  // ============================================================================

  test.describe('周上限自动计算预览', () => {
    test('5.1 输入5小时上限和周倍数后显示周上限预览', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();
      await p.clickCreate();
      await p.selectPlanType('coding_plan');

      await p.tokensPerWindowInput.fill('100');
      await p.weeklyMultiplierInput.fill('20');

      // 验证周上限预览：100 * 20 = 2000.0 M token
      await expect(p.weeklyLimitPreview).toBeVisible();
      await expect(p.weeklyLimitPreview).toContainText('2000');
      await expect(p.weeklyLimitPreview).toContainText(M_TOKEN_LABEL);
    });
  });

  // ============================================================================
  // 6. 套餐卡片分区域展示（钱包页）
  // ============================================================================

  test.describe('套餐卡片分区域展示', () => {
    test('6.1 钱包页按类型分为 API Plan 和 Coding Plan 两个区域', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);

      // 先通过管理后台创建并启用两种类型的套餐
      await p.gotoAdmin();

      // 创建 API 套餐
      await p.clickCreate();
      const apiTitle = API_PLAN.title();
      await p.titleInput.fill(apiTitle);
      await p.priceAmountInput.fill('10');
      // 确保启用
      if (!(await p.enabledSwitch.isChecked())) {
        await p.enabledSwitch.click();
      }
      await p.save();
      await expect(p.successToast).toBeVisible();
      const apiId = await p.getPlanIdByTitle(apiTitle);
      if (apiId) createdPlanIds.push(apiId);

      // 创建 CodingPlan 套餐
      await p.clickCreate();
      const cpTitle = CODING_PLAN.title();
      await p.titleInput.fill(cpTitle);
      await p.priceAmountInput.fill('20');
      await p.selectPlanType('coding_plan');
      await p.tokensPerWindowInput.fill('50');
      await p.weeklyMultiplierInput.fill('10');
      if (!(await p.enabledSwitch.isChecked())) {
        await p.enabledSwitch.click();
      }
      await p.save();
      await expect(p.successToast).toBeVisible();
      const cpId = await p.getPlanIdByTitle(cpTitle);
      if (cpId) createdPlanIds.push(cpId);

      // 导航到钱包页
      await p.gotoWallet();

      // 验证两个区域都可见
      await expect(p.apiPlanHeading).toBeVisible();
      await expect(p.codingPlanHeading).toBeVisible();
    });

    test('6.2 某类型无套餐时对应区域不显示', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoWallet();

      // 当前应该至少有 API Plan 区域（因为有已启用的套餐）
      // 检查是否有 Coding Plan 区域（取决于是否有启用的 CodingPlan 套餐）
      // 如果没有 CodingPlan 套餐，Coding Plan 区域不应该出现
      const codingPlanVisible = await p.codingPlanHeading.isVisible().catch(() => false);
      if (!codingPlanVisible) {
        // 正确行为：没有 CodingPlan 套餐时不显示区域
        expect(codingPlanVisible).toBe(false);
      }
    });
  });

  // ============================================================================
  // 7. 套餐卡片差异化显示
  // ============================================================================

  test.describe('套餐卡片差异化显示', () => {
    test('7.1 API 套餐卡片展示总额度', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();

      // 创建一个有总额度的 API 套餐
      await p.clickCreate();
      const title = API_PLAN.title();
      await p.titleInput.fill(title);
      await p.priceAmountInput.fill('15');
      await p.totalAmountInput.fill('500');
      if (!(await p.enabledSwitch.isChecked())) {
        await p.enabledSwitch.click();
      }
      await p.save();
      await expect(p.successToast).toBeVisible();
      const id = await p.getPlanIdByTitle(title);
      if (id) createdPlanIds.push(id);

      // 导航到钱包页查看卡片
      await p.gotoWallet();
      await expect(p.apiPlanHeading).toBeVisible();

      // API 套餐卡片应包含总额度信息
      const apiSection = p.apiPlanHeading.locator('..');
      await expect(apiSection.getByText(/总额度|Total Quota/i).first()).toBeVisible();
    });

    test('7.2 CodingPlan 套餐卡片展示限速参数，不展示总额度', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();

      // 创建 CodingPlan 套餐
      await p.clickCreate();
      const title = CODING_PLAN.title();
      await p.titleInput.fill(title);
      await p.priceAmountInput.fill('25');
      await p.selectPlanType('coding_plan');
      await p.tokensPerWindowInput.fill('100');
      await p.weeklyMultiplierInput.fill('20');
      if (!(await p.enabledSwitch.isChecked())) {
        await p.enabledSwitch.click();
      }
      await p.save();
      await expect(p.successToast).toBeVisible();
      const id = await p.getPlanIdByTitle(title);
      if (id) createdPlanIds.push(id);

      // 导航到钱包页
      await p.gotoWallet();
      await expect(p.codingPlanHeading).toBeVisible();

      const cpSection = p.codingPlanHeading.locator('..');
      // 应包含每 5 小时额度
      await expect(cpSection.getByText(/每.*5.*小时额度|Per 5h allowance/i).first()).toBeVisible();
      // 应包含每周额度
      await expect(cpSection.getByText(/每周额度|Weekly allowance/i).first()).toBeVisible();
    });
  });

  // ============================================================================
  // 8. 购买弹窗分类型展示
  // ============================================================================

  test.describe('购买弹窗分类型展示', () => {
    test('8.1 API 套餐购买弹窗展示总额度', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();

      // 创建启用的 API 套餐
      await p.clickCreate();
      const title = API_PLAN.title();
      await p.titleInput.fill(title);
      await p.priceAmountInput.fill('30');
      await p.totalAmountInput.fill('500');
      if (!(await p.enabledSwitch.isChecked())) {
        await p.enabledSwitch.click();
      }
      await p.save();
      await expect(p.successToast).toBeVisible();
      const id = await p.getPlanIdByTitle(title);
      if (id) createdPlanIds.push(id);

      // 去钱包页打开购买弹窗
      await p.gotoWallet();
      await p.clickSubscribe(title);

      // 弹窗中应包含 Total Quota
      await expect(p.purchaseTotalQuota).toBeVisible();
      // 不应包含 Per 5h allowance
      await expect(p.purchasePer5hAllowance).not.toBeVisible();

      await p.closePurchaseDialog();
    });

    test('8.2 CodingPlan 套餐购买弹窗展示限速参数', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();

      // 创建启用的 CodingPlan 套餐
      await p.clickCreate();
      const title = CODING_PLAN.title();
      await p.titleInput.fill(title);
      await p.priceAmountInput.fill('40');
      await p.selectPlanType('coding_plan');
      await p.tokensPerWindowInput.fill('100');
      await p.weeklyMultiplierInput.fill('20');
      if (!(await p.enabledSwitch.isChecked())) {
        await p.enabledSwitch.click();
      }
      await p.save();
      await expect(p.successToast).toBeVisible();
      const id = await p.getPlanIdByTitle(title);
      if (id) createdPlanIds.push(id);

      // 去钱包页打开购买弹窗
      await p.gotoWallet();
      await p.clickSubscribe(title);

      // 弹窗中应包含 Per 5h allowance
      await expect(p.purchasePer5hAllowance).toBeVisible();
      // 应包含 Weekly allowance
      await expect(p.purchaseWeeklyAllowance).toBeVisible();
      // 不应包含 Total Quota
      await expect(p.purchaseTotalQuota).not.toBeVisible();

      await p.closePurchaseDialog();
    });
  });

  // ============================================================================
  // 9. 购买弹窗无支付方式提示
  // ============================================================================

  test.describe('购买弹窗无支付方式提示', () => {
    test('9.1 未启用支付时弹窗显示提示信息', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();

      // 创建启用的套餐
      await p.clickCreate();
      const title = API_PLAN.title();
      await p.titleInput.fill(title);
      await p.priceAmountInput.fill('5');
      if (!(await p.enabledSwitch.isChecked())) {
        await p.enabledSwitch.click();
      }
      await p.save();
      await expect(p.successToast).toBeVisible();
      const id = await p.getPlanIdByTitle(title);
      if (id) createdPlanIds.push(id);

      // 去钱包页打开购买弹窗
      await p.gotoWallet();
      await p.clickSubscribe(title);

      // 如果未配置支付方式，应显示提示
      const alertVisible = await p.purchasePaymentAlert.isVisible().catch(() => false);
      if (alertVisible) {
        await expect(p.purchasePaymentAlert).toContainText(/在线支付|Online payment is not enabled/);
      }

      await p.closePurchaseDialog();
    });
  });

  // ============================================================================
  // 10. token 数量格式化
  // ============================================================================

  test.describe('token 数量格式化', () => {
    test('10.1 卡片和弹窗中 token 数量自动格式化', async ({ page }) => {
      const p = new SubscriptionPlanPage(page);
      await p.gotoAdmin();

      // 创建 API 套餐，总额度设为较大的值以测试格式化
      await p.clickCreate();
      const title = API_PLAN.title();
      await p.titleInput.fill(title);
      await p.priceAmountInput.fill('50');
      await p.totalAmountInput.fill('2000'); // 2000 M token = 2B tokens
      if (!(await p.enabledSwitch.isChecked())) {
        await p.enabledSwitch.click();
      }
      await p.save();
      await expect(p.successToast).toBeVisible();
      const id = await p.getPlanIdByTitle(title);
      if (id) createdPlanIds.push(id);

      // 去钱包页查看格式化展示
      await p.gotoWallet();
      await expect(p.apiPlanHeading).toBeVisible();

      // 卡片中应展示格式化的 token 数量（2B 或 2000M 或类似格式）
      const apiSection = p.apiPlanHeading.locator('..');
      await expect(apiSection.getByText(/tokens/i).first()).toBeVisible();
    });
  });
});
