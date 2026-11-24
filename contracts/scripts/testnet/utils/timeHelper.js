/**
 * 时间管理工具类
 * 处理时间相关的测试逻辑和调度
 */

class TimeHelper {
    constructor() {
        this.schedules = [];
    }

    /**
     * 获取当前时间戳（秒）
     */
    now() {
        return Math.floor(Date.now() / 1000);
    }

    /**
     * 获取未来时间戳
     * @param {number} seconds - 从现在开始的秒数
     */
    getFutureTimestamp(seconds) {
        return this.now() + seconds;
    }

    /**
     * 格式化时间戳为可读字符串
     */
    formatTimestamp(timestamp) {
        const date = new Date(timestamp * 1000);
        return date.toLocaleString("zh-CN", {
            year: "numeric",
            month: "2-digit",
            day: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
            hour12: false
        });
    }

    /**
     * 格式化持续时间
     */
    formatDuration(seconds) {
        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        const secs = seconds % 60;

        const parts = [];
        if (hours > 0) parts.push(`${hours}小时`);
        if (minutes > 0) parts.push(`${minutes}分钟`);
        if (secs > 0 || parts.length === 0) parts.push(`${secs}秒`);

        return parts.join("");
    }

    /**
     * 计算剩余时间
     */
    getTimeRemaining(targetTimestamp) {
        const remaining = targetTimestamp - this.now();
        return Math.max(0, remaining);
    }

    /**
     * 等待直到指定时间
     * @param {number} targetTimestamp - 目标时间戳
     * @param {function} callback - 可选的进度回调
     */
    async waitUntil(targetTimestamp, callback = null) {
        const startTime = this.now();
        const totalWait = targetTimestamp - startTime;

        if (totalWait <= 0) {
            console.log("  ⏭️  目标时间已过，无需等待");
            return;
        }

        console.log(`\n⏰ 等待至: ${this.formatTimestamp(targetTimestamp)}`);
        console.log(`   总等待时间: ${this.formatDuration(totalWait)}`);

        // 每10秒更新一次进度
        const updateInterval = 10;
        let lastUpdate = startTime;

        while (this.now() < targetTimestamp) {
            const remaining = this.getTimeRemaining(targetTimestamp);
            const elapsed = this.now() - startTime;

            // 每10秒或最后1秒更新进度
            if (this.now() - lastUpdate >= updateInterval || remaining <= 1) {
                const progress = ((elapsed / totalWait) * 100).toFixed(1);
                console.log(
                    `  ⏳ 进度: ${progress}% | ` +
                    `剩余: ${this.formatDuration(remaining)} | ` +
                    `${this.formatTimestamp(this.now())}`
                );
                lastUpdate = this.now();

                if (callback) {
                    await callback(elapsed, remaining);
                }
            }

            // 等待1秒
            await new Promise(resolve => setTimeout(resolve, 1000));
        }

        console.log("  ✅ 等待完成\n");
    }

    /**
     * 创建测试时间表
     */
    createSchedule(config) {
        const now = this.now();

        const schedule = {
            createdAt: now,
            projectStart: now + config.projectStartDelay,
            projectEnd: now + config.projectStartDelay + config.projectDuration,
            vestingTimes: config.vestingDelays.map(delay =>
                now + config.projectStartDelay + config.projectDuration + delay
            ),
            vestingPercents: config.vestingPercents
        };

        // 验证时间表
        this.validateSchedule(schedule);

        return schedule;
    }

    /**
     * 验证时间表的合理性
     */
    validateSchedule(schedule) {
        const now = this.now();

        // 检查项目开始时间
        if (schedule.projectStart <= now) {
            throw new Error("项目开始时间必须在未来");
        }

        // 检查项目结束时间
        if (schedule.projectEnd <= schedule.projectStart) {
            throw new Error("项目结束时间必须晚于开始时间");
        }

        // 检查 vesting 时间
        for (let i = 0; i < schedule.vestingTimes.length; i++) {
            const vestingTime = schedule.vestingTimes[i];

            if (vestingTime <= schedule.projectEnd) {
                throw new Error(`Vesting 时间 ${i + 1} 必须晚于项目结束时间`);
            }

            if (i > 0 && vestingTime <= schedule.vestingTimes[i - 1]) {
                throw new Error(`Vesting 时间必须递增`);
            }
        }

        // 检查 vesting 百分比总和
        const totalPercent = schedule.vestingPercents.reduce((a, b) => a + b, 0);
        if (totalPercent !== 10000) {
            throw new Error(`Vesting 百分比总和必须为 10000，当前为 ${totalPercent}`);
        }

        console.log("✅ 时间表验证通过");
    }

    /**
     * 打印时间表
     */
    printSchedule(schedule) {
        console.log("\n📅 测试时间表:");
        console.log("─".repeat(80));
        console.log(`创建时间:     ${this.formatTimestamp(schedule.createdAt)}`);
        console.log(`项目开始:     ${this.formatTimestamp(schedule.projectStart)}`);
        console.log(`              (${this.formatDuration(schedule.projectStart - schedule.createdAt)} 后)`);
        console.log(`项目结束:     ${this.formatTimestamp(schedule.projectEnd)}`);
        console.log(`              (持续 ${this.formatDuration(schedule.projectEnd - schedule.projectStart)})`);

        console.log("\nVesting 时间表:");
        for (let i = 0; i < schedule.vestingTimes.length; i++) {
            const vestingTime = schedule.vestingTimes[i];
            const percent = schedule.vestingPercents[i] / 100;
            const delay = vestingTime - schedule.projectEnd;

            console.log(
                `  阶段 ${i + 1}: ${this.formatTimestamp(vestingTime)} ` +
                `(结束后 ${this.formatDuration(delay)}) - ${percent}%`
            );
        }

        const totalDuration = schedule.vestingTimes[schedule.vestingTimes.length - 1] - schedule.createdAt;
        console.log(`\n总测试时长:   ${this.formatDuration(totalDuration)}`);
        console.log("─".repeat(80) + "\n");
    }

    /**
     * 注册定时任务
     */
    registerTask(timestamp, name, callback) {
        this.schedules.push({
            timestamp,
            name,
            callback,
            executed: false
        });

        // 按时间排序
        this.schedules.sort((a, b) => a.timestamp - b.timestamp);
    }

    /**
     * 执行所有待执行的任务
     */
    async executePendingTasks() {
        const now = this.now();
        const pendingTasks = this.schedules.filter(
            task => !task.executed && task.timestamp <= now
        );

        for (const task of pendingTasks) {
            console.log(`\n🔄 执行任务: ${task.name}`);
            console.log(`   计划时间: ${this.formatTimestamp(task.timestamp)}`);

            try {
                await task.callback();
                task.executed = true;
                console.log(`✅ 任务完成: ${task.name}\n`);
            } catch (error) {
                console.error(`❌ 任务失败: ${task.name}`);
                console.error(`   错误: ${error.message}\n`);
                throw error;
            }
        }

        return pendingTasks.length;
    }

    /**
     * 获取下一个待执行任务
     */
    getNextTask() {
        const pendingTasks = this.schedules.filter(task => !task.executed);
        return pendingTasks.length > 0 ? pendingTasks[0] : null;
    }

    /**
     * 打印任务列表
     */
    printTasks() {
        console.log("\n📋 任务列表:");
        console.log("─".repeat(80));

        for (let i = 0; i < this.schedules.length; i++) {
            const task = this.schedules[i];
            const status = task.executed ? "✅" : "⏳";
            const time = this.formatTimestamp(task.timestamp);

            console.log(`${status} ${i + 1}. ${task.name}`);
            console.log(`      时间: ${time}`);

            if (!task.executed) {
                const remaining = this.getTimeRemaining(task.timestamp);
                if (remaining > 0) {
                    console.log(`      剩余: ${this.formatDuration(remaining)}`);
                } else {
                    console.log(`      状态: 可执行`);
                }
            }
        }

        console.log("─".repeat(80) + "\n");
    }
}

module.exports = TimeHelper;
