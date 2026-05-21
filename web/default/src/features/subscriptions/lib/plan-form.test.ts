// [示意] 项目未配置测试框架，以下为基于 Node.js assert 的测试逻辑
// 文件：web/default/src/features/subscriptions/lib/plan-form.test.ts
import assert from 'node:assert/strict'
import { planToFormValues, formValuesToPlanPayload, PLAN_FORM_DEFAULTS } from './plan-form'
import type { SubscriptionPlan } from '../types'

function makePlan(overrides: Partial<SubscriptionPlan> = {}): SubscriptionPlan {
  return { ...PLAN_FORM_DEFAULTS, ...overrides } as SubscriptionPlan
}

// 测试1: planToFormValues 将 total_amount 从实际 token 数转为 M token
{
  const plan = makePlan({ total_amount: 500_000_000 })
  const form = planToFormValues(plan)
  assert.strictEqual(form.total_amount, 500, `total_amount 应为 500，实际为 ${form.total_amount}`)
}

// 测试2: planToFormValues 将 rate_limit_tokens_per_window 从实际 token 数转为 M token
{
  const plan = makePlan({ rate_limit_tokens_per_window: 100_000_000, plan_type: 'coding_plan' })
  const form = planToFormValues(plan)
  assert.strictEqual(form.rate_limit_tokens_per_window, 100, `rate_limit_tokens_per_window 应为 100，实际为 ${form.rate_limit_tokens_per_window}`)
}

// 测试3: planToFormValues 对 0 值保持不变
{
  const plan = makePlan({ total_amount: 0, rate_limit_tokens_per_window: 0 })
  const form = planToFormValues(plan)
  assert.strictEqual(form.total_amount, 0)
  assert.strictEqual(form.rate_limit_tokens_per_window, 0)
}

// 测试4: formValuesToPlanPayload 将 M token 转回实际 token 数（coding_plan）
{
  const payload = formValuesToPlanPayload({
    ...PLAN_FORM_DEFAULTS,
    total_amount: 500,
    rate_limit_tokens_per_window: 100,
    plan_type: 'coding_plan',
  })
  assert.strictEqual(payload.plan.total_amount, 500_000_000, `total_amount 应为 500000000，实际为 ${payload.plan.total_amount}`)
  assert.strictEqual(payload.plan.rate_limit_tokens_per_window, 100_000_000, `rate_limit_tokens_per_window 应为 100000000，实际为 ${payload.plan.rate_limit_tokens_per_window}`)
}

// 测试5: formValuesToPlanPayload 对 api 类型，rate_limit_tokens_per_window 为 0
{
  const payload = formValuesToPlanPayload({
    ...PLAN_FORM_DEFAULTS,
    total_amount: 500,
    rate_limit_tokens_per_window: 100,
    plan_type: 'api',
  })
  assert.strictEqual(payload.plan.total_amount, 500_000_000)
  assert.strictEqual(payload.plan.rate_limit_tokens_per_window, 0)
}

// 测试6: formValuesToPlanPayload 支持小数 M token（如 0.5 = 500000）
{
  const payload = formValuesToPlanPayload({
    ...PLAN_FORM_DEFAULTS,
    total_amount: 0.5,
    plan_type: 'api',
  })
  assert.strictEqual(payload.plan.total_amount, 500_000, `total_amount 应为 500000，实际为 ${payload.plan.total_amount}`)
}

// ============================================================================
// 补充测试：边界情况
// ============================================================================

// 补充1: planToFormValues 对 undefined total_amount 视为 0
{
  const plan = makePlan({ total_amount: undefined as unknown as number })
  const form = planToFormValues(plan)
  assert.strictEqual(form.total_amount, 0)
}

// 补充2: planToFormValues 对 undefined rate_limit_tokens_per_window 视为 0
{
  const plan = makePlan({ rate_limit_tokens_per_window: undefined as unknown as number })
  const form = planToFormValues(plan)
  assert.strictEqual(form.rate_limit_tokens_per_window, 0)
}

// 补充3: planToFormValues 处理非整 M 值（如 1_500_000 => 1.5）
{
  const plan = makePlan({ total_amount: 1_500_000 })
  const form = planToFormValues(plan)
  assert.strictEqual(form.total_amount, 1.5)
}

// 补充4: formValuesToPlanPayload 对 coding_plan 的 rate_limit_weekly_multiplier 保留原值
{
  const payload = formValuesToPlanPayload({
    ...PLAN_FORM_DEFAULTS,
    plan_type: 'coding_plan',
    rate_limit_weekly_multiplier: 2.5,
  })
  assert.strictEqual(payload.plan.rate_limit_weekly_multiplier, 2.5)
}

// 补充5: formValuesToPlanPayload 对 api 类型的 rate_limit_weekly_multiplier 为 0
{
  const payload = formValuesToPlanPayload({
    ...PLAN_FORM_DEFAULTS,
    plan_type: 'api',
    rate_limit_weekly_multiplier: 2.5,
  })
  assert.strictEqual(payload.plan.rate_limit_weekly_multiplier, 0)
}

// 补充6: roundtrip 一致性 - plan -> form -> payload 值一致（coding_plan）
{
  const original = makePlan({
    total_amount: 1_000_000_000,
    rate_limit_tokens_per_window: 200_000_000,
    plan_type: 'coding_plan',
  })
  const form = planToFormValues(original)
  const payload = formValuesToPlanPayload(form)
  assert.strictEqual(payload.plan.total_amount, 1_000_000_000)
  assert.strictEqual(payload.plan.rate_limit_tokens_per_window, 200_000_000)
}

// 补充7: roundtrip 一致性 - plan -> form -> payload 值一致（api 类型）
{
  const original = makePlan({
    total_amount: 750_000_000,
    rate_limit_tokens_per_window: 50_000_000,
    plan_type: 'api',
  })
  const form = planToFormValues(original)
  const payload = formValuesToPlanPayload(form)
  assert.strictEqual(payload.plan.total_amount, 750_000_000)
  assert.strictEqual(payload.plan.rate_limit_tokens_per_window, 0)
}

// 补充8: formValuesToPlanPayload 对 rate_limit_tokens_per_window 小数 M token（coding_plan）
{
  const payload = formValuesToPlanPayload({
    ...PLAN_FORM_DEFAULTS,
    rate_limit_tokens_per_window: 0.5,
    plan_type: 'coding_plan',
  })
  assert.strictEqual(payload.plan.rate_limit_tokens_per_window, 500_000)
}

// 补充9: formValuesToPlanPayload 对 total_amount 为 0 时结果为 0
{
  const payload = formValuesToPlanPayload({
    ...PLAN_FORM_DEFAULTS,
    total_amount: 0,
  })
  assert.strictEqual(payload.plan.total_amount, 0)
}

console.log('All plan-form tests passed!')

