# 实施计划：一键测试所有出站延迟功能

## 背景

用户需要在出站规则页面添加"一键测试当前页面所有出站延迟情况"的功能。当前系统仅支持单个出站测试，需要扩展为批量测试能力，同时提供清晰的进度反馈和结果汇总。

## 现有实现分析

**前端测试机制**：
- 单个测试按钮位于 [outbounds.html:99-112](web/html/settings/xray/outbounds.html)
- 测试方法 `testOutbound(index)` 在 [xray.html:655-721](web/html/xray.html)
- 使用 `outboundTestStates` 对象跟踪测试状态

**后端测试实现**：
- 核心测试方法 `TestOutbound()` 位于 [outbound.go:131-257](web/service/outbound.go)
- 使用 `testSemaphore sync.Mutex` 限制同时只能有一个测试
- 创建临时 Xray 进程，通过 SOCKS5 代理测量延迟
- API 端点：`POST /panel/xray/testOutbound`

**关键限制**：
- 当前串行测试（互斥锁），每次只能测试一个出站
- 每个测试约需 10 秒（预热 + 实际测量）
- 无批量测试 UI 和进度反馈

## 实施方案

### 1. 后端实现

#### 1.1 新增 API 路由

**文件**：[web/controller/xray_setting.go](web/controller/xray_setting.go:40)

在 `initRouter` 方法的路由组中添加：
```go
g.POST("/testAllOutbounds", a.testAllOutbounds)
```

#### 1.2 新增控制器方法

**文件**：[web/controller/xray_setting.go](web/controller/xray_setting.go)

在现有控制器方法后添加（约第 168 行后）：
```go
func (a *XraySettingController) testAllOutbounds(c *gin.Context) {
    allOutboundsJSON := c.PostForm("allOutbounds")
    if allOutboundsJSON == "" {
        jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("allOutbounds parameter is required"))
        return
    }

    results, err := a.OutboundService.TestAllOutbounds(allOutboundsJSON)
    if err != nil {
        jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
        return
    }

    jsonObj(c, results, nil)
}
```

#### 1.3 新增服务层方法

**文件**：[web/service/outbound.go](web/service/outbound.go)

添加新的结果结构体（约第 120 行后）：
```go
type TestAllOutboundResult struct {
    Index      int    `json:"index"`
    Tag        string `json:"tag"`
    Success    bool   `json:"success"`
    Delay      int64  `json:"delay"`
    Error      string `json:"error,omitempty"`
    StatusCode int    `json:"statusCode,omitempty"`
}

type TestAllOutboundsResult struct {
    Total     int                      `json:"total"`
    Tested    int                      `json:"tested"`
    Results   []*TestAllOutboundResult `json:"results"`
    TotalTime int64                    `json:"totalTime"`
}
```

添加批量测试方法（约第 257 行后）：
```go
func (s *OutboundService) TestAllOutbounds(allOutboundsJSON string) (*TestAllOutboundsResult, error) {
    startTime := time.Now()

    var allOutbounds []map[string]any
    if err := json.Unmarshal([]byte(allOutboundsJSON), &allOutbounds); err != nil {
        return nil, fmt.Errorf("invalid allOutbounds JSON: %v", err)
    }

    total := len(allOutbounds)
    results := make([]*TestAllOutboundResult, 0, total)
    tested := 0

    testURL := "https://www.google.com/generate_204" // 或从设置加载

    for i, outbound := range allOutbounds {
        tag, _ := outbound["tag"].(string)
        protocol, _ := outbound["protocol"].(string)

        // 跳过 blackhole 和 blocked 出站
        if protocol == "blackhole" || tag == "blocked" {
            results = append(results, &TestAllOutboundResult{
                Index:   i,
                Tag:     tag,
                Success: false,
                Error:   "Skipped: blocked/blackhole outbound",
            })
            tested++
            continue
        }

        // 测试单个出站（复用现有方法）
        outboundJSON, _ := json.Marshal(outbound)
        singleResult, err := s.TestOutbound(string(outboundJSON), testURL, allOutboundsJSON)

        tested++

        if err != nil {
            results = append(results, &TestAllOutboundResult{
                Index:   i,
                Tag:     tag,
                Success: false,
                Error:   err.Error(),
            })
        } else if singleResult != nil {
            results = append(results, &TestAllOutboundResult{
                Index:      i,
                Tag:        tag,
                Success:    singleResult.Success,
                Delay:      singleResult.Delay,
                Error:      singleResult.Error,
                StatusCode: singleResult.StatusCode,
            })
        }
    }

    totalTime := time.Since(startTime).Milliseconds()

    return &TestAllOutboundsResult{
        Total:     total,
        Tested:    tested,
        Results:   results,
        TotalTime: totalTime,
    }, nil
}
```

**设计要点**：
- 复用现有的 `TestOutbound()` 方法，保持并发控制
- 自动跳过 blackhole 和 blocked 类型出站
- 串行测试，避免资源竞争
- 返回完整结果集和总测试时间

### 2. 前端实现

#### 2.1 添加 UI 组件

**文件**：[web/html/settings/xray/outbounds.html](web/html/settings/xray/outbounds.html)

在第 10-13 行（现有按钮区域）添加批量测试按钮：
```html
<a-button
    type="primary"
    icon="thunderbolt"
    @click="testAllOutbounds"
    :loading="batchTesting"
    :disabled="batchTesting">
    <span v-if="!batchTesting">{{ i18n "pages.xray.outbound.testAll" }}</span>
    <span v-else>[[ batchTestProgress ]]</span>
</a-button>
```

在第 15 行后（操作按钮区域后）添加进度显示：
```html
<a-alert
    v-if="batchTesting"
    type="info"
    show-icon
    style="margin-bottom: 16px;">
    <template slot="message">
        <span>{{ i18n "pages.xray.outbound.testingAll" }}</span>
        <a-progress
            :percent="batchTestProgressPercent"
            :status="batchTestStatus"
            :format="() => `${batchTestProgress.current}/${batchTestProgress.total}`" />
    </template>
</a-alert>
```

添加结果汇总组件：
```html
<a-alert
    v-if="batchTestResults"
    type="success"
    show-icon
    closable
    @close="clearBatchTestResults"
    style="margin-bottom: 16px;">
    <template slot="message">
        <div>{{ i18n "pages.xray.outbound.testComplete" }}</div>
        <a-statistic-group style="margin-top: 8px;">
            <a-statistic :title="i18n('pages.xray.outbound.testTotal')" :value="batchTestResults.total" />
            <a-statistic :title="i18n('pages.xray.outbound.testSuccess')" :value="batchTestResults.successCount" :value-style="{ color: '#52c41a' }" />
            <a-statistic :title="i18n('pages.xray.outbound.testFailed')" :value="batchTestResults.failedCount" :value-style="{ color: '#f5222d' }" />
            <a-statistic :title="i18n('pages.xray.outbound.testTime')" :value="batchTestResults.totalTime" suffix="ms" />
        </a-statistic-group>
    </template>
</a-alert>
```

#### 2.2 添加数据和方法

**文件**：[web/html/xray.html](web/html/xray.html)

在 Vue data 中添加（约第 260 行后）：
```javascript
data: {
    // 现有数据...

    // 批量测试状态
    batchTesting: false,
    batchTestProgress: {
        current: 0,
        total: 0,
        message: ''
    },
    batchTestProgressPercent: 0,
    batchTestStatus: 'active',
    batchTestResults: null,
}
```

在 methods 中添加（约第 721 行后）：
```javascript
async testAllOutbounds() {
    if (!this.templateSettings.outbounds || this.templateSettings.outbounds.length === 0) {
        Vue.prototype.$message.warning('No outbounds to test');
        return;
    }

    const testableOutbounds = this.templateSettings.outbounds.filter(outbound =>
        outbound.protocol !== 'blackhole' && outbound.tag !== 'blocked'
    );

    if (testableOutbounds.length === 0) {
        Vue.prototype.$message.warning('No testable outbounds found');
        return;
    }

    // 大量出站时确认
    if (testableOutbounds.length > 10) {
        const confirmed = await this.$confirm({
            title: 'Confirm batch test',
            content: `You are about to test ${testableOutbounds.length} outbounds. This may take several minutes. Continue?`,
            okText: 'Confirm',
            cancelText: 'Cancel',
        });
        if (!confirmed) return;
    }

    this.batchTesting = true;
    this.batchTestProgress = {
        current: 0,
        total: testableOutbounds.length,
        message: 'Starting batch test...'
    };
    this.batchTestProgressPercent = 0;
    this.batchTestStatus = 'active';
    this.batchTestResults = null;

    this.outboundTestStates = {};

    try {
        const allOutboundsJSON = JSON.stringify(this.templateSettings.outbounds || []);

        const msg = await HttpUtil.post("/panel/xray/testAllOutbounds", {
            allOutbounds: allOutboundsJSON
        });

        if (!msg.success) {
            throw new Error(msg.msg || 'Batch test failed');
        }

        const result = msg.obj;

        if (result.results && Array.isArray(result.results)) {
            result.results.forEach((testResult, idx) => {
                const actualIndex = testResult.index;
                this.$set(this.outboundTestStates, actualIndex, {
                    testing: false,
                    result: {
                        success: testResult.success,
                        delay: testResult.delay,
                        error: testResult.error,
                        statusCode: testResult.statusCode
                    }
                });

                this.batchTestProgressPercent = Math.round(((idx + 1) / result.results.length) * 100);
            });
        }

        const successCount = result.results.filter(r => r.success).length;
        const failedCount = result.results.filter(r => !r.success && r.error && !r.error.includes('Skipped')).length;

        this.batchTestResults = {
            total: result.total,
            tested: result.tested,
            successCount,
            failedCount,
            totalTime: result.totalTime,
            results: result.results
        };

        this.batchTestStatus = failedCount === 0 ? 'success' : 'exception';

        if (failedCount === 0) {
            Vue.prototype.$message.success(`Successfully tested ${successCount} outbounds in ${result.totalTime}ms`);
        } else {
            Vue.prototype.$message.warning(`Test completed: ${successCount} succeeded, ${failedCount} failed. Time: ${result.totalTime}ms`);
        }

    } catch (error) {
        Vue.prototype.$message.error('Batch test failed: ' + error.message);
        this.batchTestStatus = 'exception';
    } finally {
        this.batchTesting = false;
    }
},

clearBatchTestResults() {
    this.batchTestResults = null;
}
```

#### 2.3 添加国际化字符串

**文件**：[web/locale/zh-cn.yml](web/locale/zh-cn.yml) 和 [web/locale/en-us.yml](web/locale/en-us.yml)

添加以下翻译键：
```yaml
pages.xray.outbound.testAll: "测试全部" / "Test All"
pages.xray.outbound.testingAll: "正在测试所有出站..." / "Testing all outbounds..."
pages.xray.outbound.testComplete: "批量测试完成" / "Batch test completed"
pages.xray.outbound.testTotal: "总数" / "Total"
pages.xray.outbound.testSuccess: "成功" / "Success"
pages.xray.outbound.testFailed: "失败" / "Failed"
pages.xray.outbound.testTime: "耗时" / "Time"
```

### 3. 关键文件清单

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| [web/service/outbound.go](web/service/outbound.go) | 新增方法 | 添加 `TestAllOutbounds()` 方法和结果结构体 |
| [web/controller/xray_setting.go](web/controller/xray_setting.go) | 新增路由和方法 | 添加 `/testAllOutbounds` 端点 |
| [web/html/settings/xray/outbounds.html](web/html/settings/xray/outbounds.html) | 新增 UI | 添加批量测试按钮、进度显示和结果汇总 |
| [web/html/xray.html](web/html/xray.html) | 新增方法 | 添加 `testAllOutbounds()` 方法和状态数据 |
| [web/locale/zh-cn.yml](web/locale/zh-cn.yml) | 新增翻译 | 添加批量测试相关翻译 |
| [web/locale/en-us.yml](web/locale/en-us.yml) | 新增翻译 | 添加批量测试相关翻译 |

### 4. 性能考虑

**估算测试时间**：
- 每个出站约 10 秒
- 10 个出站：约 100 秒（1.7 分钟）
- 50 个出站：约 500 秒（8.3 分钟）

**优化措施**：
- 自动跳过不可测试类型（blackhole、blocked）
- 大量出站时（>10）显示确认对话框
- 实时进度反馈改善用户体验
- 结果汇总便于快速查看

**后续优化方向**（可选）：
- 将 `testSemaphore` 改为缓冲 channel，支持 2-3 个并发测试
- 使用 WebSocket 或 SSE 实现真正的实时进度推送
- 添加测试取消功能

### 5. 测试验证

#### 功能测试
1. **少量出站**（2-5 个）：验证基本功能和结果显示
2. **中等数量**（10-20 个）：验证进度显示和性能
3. **混合协议**：包含 vmess、vless、trojan、shadowsocks 等
4. **边界情况**：
   - 空出站列表
   - 全部为 blackhole
   - 部分出站不可达

#### 验证步骤
1. 启动应用，进入出站规则页面
2. 添加多个测试出站（至少 3 个不同协议）
3. 点击"测试全部"按钮
4. 观察进度条和测试状态
5. 验证测试结果与单个测试结果一致
6. 检查结果汇总统计数据准确性

#### 回归测试
- 确保单个测试功能仍然正常工作
- 确认其他出站操作（添加、编辑、删除）不受影响
- 验证页面性能（无内存泄漏、UI 卡顿）

## 注意事项

1. **并发控制**：保持现有串行测试机制，避免资源竞争
2. **超时处理**：可能需要调整服务器超时配置（大量出站时）
3. **错误处理**：妥善处理网络错误、超时等异常情况
4. **向后兼容**：不影响现有单个测试功能
5. **用户体验**：清晰的状态提示和结果展示
