/**
 * 测试数据常量
 * 数据分类：
 * - A 类：测试者控制，无唯一性 → 静态常量
 * - B 类：测试者控制，需唯一性 → uniqueSuffix()
 */

let counter = 0;
function uniqueSuffix(): string {
  return `_${Date.now()}_${++counter}`;
}

export const UNIQUE_SUFFIX = uniqueSuffix;

// API 套餐测试数据
export const API_PLAN = {
  title: () => `E2E API Plan${uniqueSuffix()}`,
  subtitle: 'E2E test API plan',
  planType: 'api' as const,
  priceAmount: 9.99,
  durationUnit: 'month' as const,
  durationValue: 1,
  upgradeGroup: '',
  purchaseLimit: 0,
  sortOrder: 0,
  enabled: true,
  totalAmount: 500, // M token
  resetPeriod: 'never' as const,
};

// CodingPlan 套餐测试数据
export const CODING_PLAN = {
  title: () => `E2E Coding Plan${uniqueSuffix()}`,
  subtitle: 'E2E test coding plan',
  planType: 'coding_plan' as const,
  priceAmount: 20.0,
  durationUnit: 'month' as const,
  durationValue: 1,
  upgradeGroup: '',
  purchaseLimit: 0,
  sortOrder: 0,
  enabled: true,
  tokensPerWindow: 100, // M token
  weeklyMultiplier: 20,
};

// 必填校验用
export const REQUIRED_FIELD_ERRORS = {
  title: 'Plan Title',
  priceAmount: 'Actual Amount',
};

// M token 单位文本
export const M_TOKEN_LABEL = 'M token';
export const M_TOKEN_PLACEHOLDER = 'Enter amount in M tokens';

// 周上限预览文本
export const WEEKLY_LIMIT_PREFIX = 'Weekly limit';
