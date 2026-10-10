export type QualityManualMethod = 'questions' | 'manxue_candy' | 'manxue_pelican'

export interface QualityQuestion {
  id: string
  name: string
  enabled: boolean
  prompt: string
  answer: string
  matchMode: 'answer' | 'keyword' | 'regex'
  maxDurationMs: number
}
export interface QualitySettings {
  detectionMethod: 'questions' | 'manxue'
  manxueBenchmark: 'candy' | 'pelican'
  manxueProtocol: 'responses' | 'chat_completions'
  manxueServiceTier: '' | 'priority' | 'ultrafast'
  enabled: boolean
  revision: string
  model: string
  reasoningEffort: '' | 'low' | 'medium' | 'high' | 'xhigh' | 'max' | 'ultra'
  mode: 'content' | 'time' | 'content_time'
  intervalSeconds: number
  retrySeconds: number
  failureLimit: number
  recoveryLimit: number
  concurrency: number
  timeoutSeconds: number
  maxTokens: number
  historyLimit: number
  questions: QualityQuestion[]
}
export interface QualityChannel {
  targetId: string
  enabled: boolean
  independent: boolean
}
export interface QualityGroup {
  groupId: string
  enabled: boolean
  globalEnabled: boolean
  errorKey?: string
}
export interface QualitySample {
  manual?: boolean
  startedAt?: string
  prompt?: string
  mode?: 'content' | 'time' | 'content_time'
  html?: string
  hasHtml?: boolean
  htmlTooLarge?: boolean
  detectionMethod?: 'questions' | 'manxue'
  benchmark?: 'candy' | 'pelican'
  report?: string
  id: string
  targetId: string
  model: string
  questionId: string
  questionName: string
  answer: string
  expectedAnswer: string
  matchMode: string
  result: 'passed' | 'failed' | 'error'
  errorKey?: string
  contentPassed: boolean
  timePassed: boolean
  durationMs: number
  maxDurationMs: number
  createdAt: string
}
export interface QualityState {
  targetId: string
  revision: string
  status: 'normal' | 'suspect' | 'degraded' | 'recovering' | 'error'
  degraded: boolean
  failures: number
  successes: number
  nextQuestionId: string
  nextProbeAt: string
  latest: QualitySample
}
