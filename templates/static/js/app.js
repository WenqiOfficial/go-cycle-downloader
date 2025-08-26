$(document).ready(function () {
    // 页面加载时获取初始状态，并根据状态决定是否开启进度轮询
    setTimeout(autoLoad, 1000);

    // 启动定期状态刷新（每5秒），但只刷新状态，不刷新配置
    setInterval(autoLoadStatus, 5000);

    // 绑定表单提交事件
    $('#setForm').on('submit', function (e) {
        e.preventDefault();
        var fd = new FormData(this);
        $.ajax({
            url: '/api/set',
            type: 'POST',
            data: fd,
            processData: false,
            contentType: false,
            success: function(data) {
                updateAll(data); // 保存配置后完整更新
                // 显示保存成功提示
                showMessage('配置保存成功', 'success');
            },
            error: function() {
                showMessage('配置保存失败', 'error');
            }
        });
    });

    // 绑定计划类型切换事件
    $('#planType').on('change', togglePlanType);
});

// --- 消息提示功能 ---

let messageTimeout = null;

// 显示消息提示
function showMessage(message, type = 'info') {
    const messageArea = $('#messageArea');
    const messageText = $('#messageText');
    
    // 清除之前的超时
    if (messageTimeout) {
        clearTimeout(messageTimeout);
    }
    
    // 如果消息已经显示，先隐藏
    if (messageArea.hasClass('show')) {
        hideMessage(function() {
            // 隐藏完成后再显示新消息
            setTimeout(() => showNewMessage(message, type), 100);
        });
        return;
    }
    
    showNewMessage(message, type);
}

// 显示新消息的内部函数
function showNewMessage(message, type) {
    const messageArea = $('#messageArea');
    const messageText = $('#messageText');
    
    // 清除之前的类型和动画类
    messageArea.removeClass('alert-success alert-danger alert-warning alert-info animate-in animate-out');
    
    // 添加新的类型
    switch(type) {
        case 'success':
            messageArea.addClass('alert-success');
            break;
        case 'error':
        case 'danger':
            messageArea.addClass('alert-danger');
            break;
        case 'warning':
            messageArea.addClass('alert-warning');
            break;
        default:
            messageArea.addClass('alert-info');
    }
    
    messageText.text(message);
    
    // 显示消息并添加进入动画
    messageArea.show().addClass('show animate-in');
    
    // 移除动画类
    setTimeout(() => {
        messageArea.removeClass('animate-in');
    }, 300);
    
    // 4秒后自动隐藏
    messageTimeout = setTimeout(hideMessage, 4000);
}

// 隐藏消息提示
function hideMessage(callback) {
    const messageArea = $('#messageArea');
    
    if (!messageArea.hasClass('show')) {
        if (callback) callback();
        return;
    }
    
    // 添加退出动画
    messageArea.addClass('animate-out');
    
    // 动画完成后隐藏元素
    setTimeout(() => {
        messageArea.removeClass('show animate-out').hide();
        if (callback) callback();
    }, 300);
}

// --- 按钮点击处理 ---

// 立即下载
function downloadFile() {
    $.post('/api/download', function(data) {
        updateStatus(data); // 只更新状态，不更新配置
        showMessage('下载任务已启动', 'success');
        // 开始轮询进度
        pollProgress();
    }).fail(function(jqXHR) {
        showMessage('启动下载失败', 'error');
        if (jqXHR.responseJSON) {
            updateStatus(jqXHR.responseJSON);
        }
    });
}

// 清理缓存
function cleanCache() {
    $.post('/api/clean', function(data) {
        updateStatus(data);
        showMessage('缓存清理任务已启动', 'success');
    }).fail(function() {
        showMessage('清理缓存失败', 'error');
    });
}

// 停止下载
function stopDownload() {
    $.post('/api/stop', function(data) {
        updateStatus(data);
        showMessage('已发送停止信号', 'warning');
    }).fail(function() {
        showMessage('停止下载失败', 'error');
    });
}

// 切换任务状态
function toggleTask() {
    $.post('/api/toggle_task', function(data) {
        updateStatus(data);
        const message = data.task_enabled ? '自动任务已启用' : '自动任务已暂停';
        showMessage(message, 'success');
    }).fail(function() {
        showMessage('切换任务状态失败', 'error');
    });
}

// 切换下载量限制
function toggleLimit() {
    $.post('/api/toggle_limit', function(data) {
        updateStatus(data);
        const message = data.config.daily_limit_enabled ? '下载量限制已启用' : '下载量限制已关闭';
        showMessage(message, 'success');
    }).fail(function() {
        showMessage('切换限制状态失败', 'error');
    });
}

// --- UI 逻辑 ---

// 更新任务按钮状态
function updateTaskButton(enabled) {
    const btn = $('#toggleTaskBtn');
    if (enabled) {
        btn.text('⏸️ 暂停自动任务')
           .removeClass('btn-outline-success btn-outline-danger')
           .addClass('btn-outline-danger');
    } else {
        btn.text('▶️ 启用自动任务')
           .removeClass('btn-outline-success btn-outline-danger')
           .addClass('btn-outline-success');
    }
}

// 更新限制按钮状态
function updateLimitButton(enabled) {
    const btn = $('#toggleLimitBtn');
    if (enabled) {
        btn.text('❌ 关闭下载量限制')
           .removeClass('btn-outline-warning btn-outline-secondary')
           .addClass('btn-outline-secondary');
    } else {
        btn.text('📊 启用下载量限制')
           .removeClass('btn-outline-warning btn-outline-secondary')
           .addClass('btn-outline-warning');
    }
}

// 根据计划类型显示/隐藏表单项
function togglePlanType() {
    var type = $('#planType').val();
    if (type === 'daily') {
        $('#intervalGroup').addClass('d-none');
        $('#hourGroup').removeClass('d-none');
        $('#minuteGroup').removeClass('d-none');
    } else {
        $('#intervalGroup').removeClass('d-none');
        $('#hourGroup').addClass('d-none');
        $('#minuteGroup').addClass('d-none');
    }
}

// --- 数据与状态更新 ---

// 使用服务器返回的数据更新整个页面（包括配置区）
function updateAll(data) {
    if (!data) return;
    updateConfig(data);
    updateStatus(data);
}

// 只更新配置区域
function updateConfig(data) {
    if (!data) return;

    // 配置区
    $('#urlInput').val(data.config.url || '');
    $('#planType').val(data.config.plan_type || 'interval');
    togglePlanType();
    $('#intervalInput').val(data.config.interval_minutes || 60);
    $('#hourInput').val(data.config.hour || 0);
    $('#minuteInput').val(data.config.minute || 0);
    $('#speedInput').val(data.config.speed_kb || 0);
    $('#dirInput').val(data.config.dir || '');
    $('#limitInput').val(data.config.limit_mb || 100);
}

// 只更新状态区域（不更新配置）
function updateStatus(data) {
    if (!data) return;

    // 状态区
    $('#dirText').text(data.config.dir || '-');
    
    // 任务运行状态加上颜色指示
    const taskStatus = data.task_status || '-';
    const statusElement = $('#taskStatus');
    statusElement.text(taskStatus);
    statusElement.removeClass('text-success text-danger text-warning text-primary');

    switch(taskStatus)
    {
        case '下载中':
            statusElement.addClass('text-primary');
            break;
        case '失败':
            statusElement.addClass('text-danger');
            break;
        case '空闲':
            statusElement.addClass('text-success');
            break;
        default:
    }

    const taskEnabled = $('#taskEnabled');
    
    taskEnabled.text(data.task_enabled ? '已启用' : '已禁用');
    if (data.task_enabled) {
        taskEnabled.removeClass('text-danger').addClass('text-success');
    } else {
        taskEnabled.removeClass('text-success').addClass('text-danger');
    }

    // 改进执行计划的显示
    let planText = '';
    if (data.config.plan_type === 'daily') {
        planText = `每天 ${String(data.config.hour || 0).padStart(2, '0')}:${String(data.config.minute || 0).padStart(2, '0')} 执行`;
    } else {
        planText = `每隔 ${data.config.interval_minutes || 60} 分钟执行`;
    }
    $('#planTypeText').text(planText);

    const limitEnabled = $('#limitEnabled');
    limitEnabled.text(data.config.daily_limit_enabled ? '已启用' : '已禁用');
    if (data.config.daily_limit_enabled) {
        limitEnabled.removeClass('text-danger').addClass('text-success');
    } else {
        limitEnabled.removeClass('text-success').addClass('text-danger');
    }
    $('#limitMB').text((data.config.limit_mb || 0) + ' MB');
    $('#todayMB').text((data.stats.daily_downloaded_mb || 0) + ' MB');
    $('#monthMB').text((data.stats.monthly_downloaded_mb || 0) + ' MB');
    
    // 改进按钮文本和样式
    updateTaskButton(data.task_enabled);
    updateLimitButton(data.config.daily_limit_enabled);

    // 下载状态区
    $('#lastDownload').text(data.stats.last_download || '-');
    $('#lastFile').text(data.stats.last_file || '-');
    $('#msg').text(data.stats.message || '-');

    // 如果从/status接口获取的状态是“下载中”，则启动轮询
    if (data.task_status === '下载中') {
        pollProgress();
    }
}

let progressTimer = null;
let statusTimer = null;

// 轮询进度接口
function pollProgress() {
    // 如果已有计时器，先清除，避免重复
    if (progressTimer) {
        clearInterval(progressTimer);
    }

    progressTimer = setInterval(() => {
        $.getJSON('/api/progress', (data) => {
            if (!data) return;
            let percent = data.percent || 0;
            let speed = data.speed || 0;
            let size = data.size || 0;
            let status = data.status || '';

            // 格式化文件大小显示
            let sizeText = '';
            if (size > 1024) {
                sizeText = (size / 1024).toFixed(1) + ' MB';
            } else if (size > 0) {
                sizeText = size + ' KB';
            } else {
                sizeText = '-';
            }

            // 更新进度UI
            $('#progressText').text(percent + '%');
            $('#speedText').text(speed + ' KB/s');
            $('#sizeText').text(sizeText);
            
            // 更新进度条，添加颜色变化
            const progressBar = $('#progressBar');
            progressBar.css('width', percent + '%').text(percent + '%');
            
            // 根据状态改变进度条颜色
            progressBar.removeClass('bg-success bg-danger bg-warning');
            if (status.includes('失败') || status.includes('错误')) {
                progressBar.addClass('bg-danger');
            } else if (status.includes('完成')) {
                progressBar.addClass('bg-success');
            } else if (status.includes('停止')) {
                progressBar.addClass('bg-warning');
            }

            // 当下载完成、失败、停止或进入空闲时，停止轮询
            if (percent >= 100 || status === '下载完成' || status === '下载失败' || status === '已手动停止' || status === '空闲') {
                clearInterval(progressTimer);
                progressTimer = null;
                // 延迟一点时间后获取最终状态，确保后端已更新完毕
                setTimeout(autoLoadStatus, 1500); // 只更新状态，不更新配置
            }
        }).fail(() => {
            // 请求失败也停止轮询
            clearInterval(progressTimer);
            progressTimer = null;
        });
    }, 1000); // 轮询间隔为1秒
}

// 页面加载时自动刷新配置和状态（完整加载）
function autoLoad() {
    $.getJSON('/api/status', function(data) {
        updateAll(data); // 更新配置和状态
        // 如果当前有下载任务在进行，立即开始轮询进度
        if (data && data.task_status === '下载中') {
            if (!progressTimer) {
                pollProgress();
            }
        } else {
            // 如果没有下载任务，停止进度轮询
            if (progressTimer) {
                clearInterval(progressTimer);
                progressTimer = null;
            }
        }
    }).fail(function() {
        console.log('获取状态失败，将重试...');
    });
}

// 只刷新状态信息，不刷新配置区域
function autoLoadStatus() {
    $.getJSON('/api/status', function(data) {
        updateStatus(data); // 只更新状态
        // 如果当前有下载任务在进行，立即开始轮询进度
        if (data && data.task_status === '下载中') {
            if (!progressTimer) {
                pollProgress();
            }
        } else {
            // 如果没有下载任务，停止进度轮询
            if (progressTimer) {
                clearInterval(progressTimer);
                progressTimer = null;
            }
        }
    }).fail(function() {
        console.log('获取状态失败，将重试...');
    });
}
