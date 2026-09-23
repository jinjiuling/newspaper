// ==UserScript==
// @name         公需课16倍速连播助手（sdcme.net.cn专用）
// @namespace    http://tampermonkey.net/
// @version      7.0
// @description  强制破解山东公需课学习平台倍速限制 + 反作弊绕过 + 自动连播未学课程
// @author       You
// @match        *://course.sdcme.net.cn/*
// @grant        none
// @run-at       document-idle
// ==/UserScript==

(function () {
    'use strict';

    // ==================== 配置区 ====================
    const TARGET_SPEED = 16;           // 目标倍速
    const MAX_SPEED = 16;              // 浏览器最大倍速
    const SPEED_OPTIONS = [0.5, 1, 1.5, 2, 3, 4, 8, 12, 16];
    const HOTKEY_DEC = '[';
    const HOTKEY_INC = ']';
    const SEEK_BACK_LIMIT = 3600;        // 拦截往回seek阈值（秒），设大一点，不管往回拉多少都拦
    const AUTO_PLAY = true;             // 自动连播下一个
    const END_THRESHOLD = 5;            // 距离结尾多少秒算播完（秒）
    // =================================================

    let currentSpeed = TARGET_SPEED;
    let panel = null;
    let speedLabel = null;
    let speedInput = null;
    let nextBtn = null;

    const isPlayPage = location.pathname.includes('/play/');
    const isCoursePage = location.pathname.match(/\/course\/\d+/);

    console.log('[倍速助手] 脚本启动，页面类型:', isCoursePage ? '课程页' : isPlayPage ? '播放页' : '其他');

    // ---------- 工具函数 ----------
    function getPlayer() {
        const video = document.querySelector('video');
        if (video && video.player && typeof video.player.setSpeed === 'function') {
            return video.player;
        }
        return null;
    }

    function getCurrentVid() {
        const match = location.search.match(/[?&]vid=([^&]+)/);
        return match ? match[1] : null;
    }

    // ---------- 反作弊绕过 ----------
    function bypassAntiCheat() {
        const player = getPlayer();
        if (!player) return;

        // 如果 _doSeek 还不存在，跳过，等下次再试
        if (typeof player._doSeek !== 'function') {
            return false;
        }

        // 已经hook过了就不重复hook
        if (player._doSeek._hooked) {
            return true;
        }

        const origDoSeek = player._doSeek;
        player._seekBlocked = 0;

        const hooked = function (pos) {
            // 每次都重新获取video，避免闭包捕获的旧引用失效
            const video = this.video || document.querySelector('video');
            if (!video) return origDoSeek.call(this, pos);
            
            const currentPos = video.currentTime;
            if (pos < currentPos && (currentPos - pos) < SEEK_BACK_LIMIT) {
                player._seekBlocked++;
                return;
            }
            return origDoSeek.call(this, pos);
        };
        hooked._hooked = true;
        player._doSeek = hooked;
        console.log('[倍速助手] 反作弊绕过已启用，已hook _doSeek');
        return true;
    }

    // 定期检查hook是否还在，被覆盖了就重新hook
    function startHookWatcher() {
        setInterval(() => {
            const player = getPlayer();
            if (player && !player._doSeek?._hooked) {
                console.log('[倍速助手] hook被覆盖，重新hook...');
                bypassAntiCheat();
            }
        }, 2000);
    }

    // ---------- 设置倍速 ----------
    function applySpeed(speed) {
        const player = getPlayer();
        if (player) {
            try { player.setSpeed(speed); return true; } catch (e) {}
        }
        return false;
    }

    // ---------- 课程页：抓取视频列表，存到 localStorage ----------
    function scrapeCourseList() {
        const links = document.querySelectorAll('a[href*="/play/"]');
        const videos = Array.from(links).map(a => {
            const parent = a.parentElement;
            const vid = a.href.match(/[?&]vid=([^&]+)/)?.[1];
            const title = decodeURIComponent(a.href.match(/title=([^&]+)/)?.[1] || '');

            // 判断是否已学：前面的序号方块是绿色对勾
            const badge = parent.querySelector('span');
            const isDone = badge && badge.className.includes('bg-emerald-100');

            return { vid, title, isDone };
        }).filter(v => v.vid);

        // 按课程ID存
        const courseId = location.pathname.match(/\/course\/(\d+)/)?.[1];
        if (courseId && videos.length > 0) {
            localStorage.setItem('sdcme_video_list_' + courseId, JSON.stringify(videos));
            console.log('[倍速助手] 已保存', videos.length, '个视频到本地，课程ID:', courseId);
            return { courseId, videos };
        }
        return null;
    }

    // ---------- 播放页：自动从课程页抓取视频列表 ----------
    async function fetchVideoListAuto() {
        if (nextBtn) nextBtn.textContent = '正在获取课程列表...';

        try {
            // 1. 请求首页，拿到所有课程链接
            const homeRes = await fetch('/');
            const homeHtml = await homeRes.text();
            const courseLinks = [...homeHtml.matchAll(/href="\/course\/(\d+)"/g)].map(m => m[1]);
            const uniqueCourses = [...new Set(courseLinks)];
            console.log('[倍速助手] 发现', uniqueCourses.length, '个课程');

            // 2. 依次请求每个课程页，找当前vid在哪个课程里
            const currentVid = getCurrentVid();
            for (const courseId of uniqueCourses) {
                const courseRes = await fetch('/course/' + courseId);
                const courseHtml = await courseRes.text();

                // 找所有视频链接
                const videoLinks = [...courseHtml.matchAll(/href="\/play\/index\.html\?vid=([^&]+)&title=([^"]+)"/g)];
                const videos = videoLinks.map(m => {
                    const vid = m[1];
                    const title = decodeURIComponent(m[2]);
                    // 判断是否已学：看有没有绿色对勾的样式
                    const isDone = courseHtml.includes('bg-emerald-100') && courseHtml.indexOf(vid) > courseHtml.indexOf('bg-emerald-100');
                    return { vid, title, isDone };
                });

                // 存到 localStorage
                if (videos.length > 0) {
                    localStorage.setItem('sdcme_video_list_' + courseId, JSON.stringify(videos));
                    console.log('[倍速助手] 已保存课程', courseId, '的', videos.length, '个视频');
                }

                // 找到当前vid所在的课程，就可以停了
                if (videos.some(v => v.vid === currentVid)) {
                    console.log('[倍速助手] 找到当前视频所属课程:', courseId);
                    return videos;
                }
            }
        } catch (e) {
            console.error('[倍速助手] 自动获取列表失败:', e);
            if (nextBtn) nextBtn.textContent = '获取列表失败，请访问课程页';
        }
        return null;
    }

    // ---------- 播放页：打印当前连播状态 ----------
    function logPlaybackStatus() {
        const currentVid = getCurrentVid();
        console.log('========== 倍速助手状态 ==========');
        console.log('当前视频VID:', currentVid);

        const allKeys = Object.keys(localStorage).filter(k => k.startsWith('sdcme_video_list_'));
        console.log('本地缓存课程列表数:', allKeys.length);
        console.log('缓存的key:', allKeys);

        if (allKeys.length === 0) {
            console.log('⚠️ 没有视频列表，请先访问课程详情页');
            return;
        }

        for (const key of allKeys) {
            const list = JSON.parse(localStorage.getItem(key));
            const idx = list.findIndex(v => v.vid === currentVid);
            console.log(`课程 ${key}: 共 ${list.length} 个视频`);
            if (idx >= 0) {
                console.log(`  ✅ 当前在第 ${idx + 1}/${list.length} 个: ${list[idx].title}`);
                let nextCount = 0;
                for (let i = idx + 1; i < list.length; i++) {
                    if (!list[i].isDone) {
                        console.log(`  ⏭️ 第${i + 1}个未学: ${list[i].title.slice(0, 40)}`);
                        nextCount++;
                        if (nextCount >= 3) break;
                    }
                }
            } else {
                console.log(`  ⚠️ 列表中未找到当前视频`);
            }
        }
        console.log('==================================');
    }

    // ---------- 播放页：自动连播逻辑 ----------
    async function autoPlayNext(video) {
        if (!AUTO_PLAY) return;
        console.log('\\n[连播] ====== 开始查找下一个视频 ======');

        let allKeys = Object.keys(localStorage).filter(k => k.startsWith('sdcme_video_list_'));
        console.log('[连播] 本地缓存列表数:', allKeys.length);
        console.log('[连播] 缓存的key:', allKeys);

        if (allKeys.length === 0) {
            console.log('[连播] 本地没有视频列表，自动获取...');
            await fetchVideoListAuto();
            allKeys = Object.keys(localStorage).filter(k => k.startsWith('sdcme_video_list_'));
            console.log('[连播] 自动获取后列表数:', allKeys.length);
        }

        if (allKeys.length === 0) {
            console.log('[连播] ❌ 无法获取列表');
            if (nextBtn) nextBtn.textContent = '无法获取列表，请访问课程页';
            return;
        }

        const currentVid = getCurrentVid();
        for (const key of allKeys) {
            const list = JSON.parse(localStorage.getItem(key));
            console.log(`[连播] 检查课程 ${key}: 共${list.length}个视频`);
            const idx = list.findIndex(v => v.vid === currentVid);
            if (idx >= 0) {
                console.log(`[连播] ✅ 找到当前视频在第 ${idx + 1}/${list.length} 位`);
                for (let i = idx + 1; i < list.length; i++) {
                    console.log(`[连播]   第${i + 1}个: ${list[i].title.slice(0, 30)} (已学: ${list[i].isDone})`);
                    if (!list[i].isDone) {
                        console.log(`[连播] 🎯 下一个未学视频: ${list[i].title}`);
                        setTimeout(() => {
                            console.log(`[连播] 🔗 2秒后跳转到: ${list[i].vid}`);
                            location.href = '/play/index.html?vid=' + list[i].vid + '&title=' + encodeURIComponent(list[i].title);
                        }, 2000);
                        updateNextBtn(`2秒后跳: ${list[i].title.slice(0, 10)}...`);
                        return;
                    }
                }
                console.log('[连播] 🎉 全部学完了');
                updateNextBtn('全部学完 ✓');
                return;
            } else {
                console.log(`[连播]   当前视频不在这个课程里`);
            }
        }
        console.log('[连播] ❌ 当前vid不在任何列表中');
    }

    // ---------- 自动播放 ----------
    function autoPlayVideo() {
        const player = getPlayer();
        if (!player) return;

        const video = player.video || document.querySelector('video');
        if (!video) {
            console.log('[倍速助手] 找不到video元素，稍后再试');
            setTimeout(autoPlayVideo, 1000);
            return;
        }

        console.log('[倍速助手] 尝试自动播放...');

        // 先调用 player.play()
        try {
            player.play();
        } catch (e) {}

        // 如果还在暂停，说明被浏览器自动播放策略拦截了
        setTimeout(() => {
            const v = player.video || document.querySelector('video');
            if (v && v.paused) {
                console.log('[倍速助手] 被浏览器自动播放策略拦截，等待用户首次点击解锁');
                // 提示用户点击一下页面任意位置解锁
                showUnlockHint();
                // 监听用户第一次点击/按键，解锁后自动播放
                const unlock = () => {
                    console.log('[倍速助手] 用户已交互，自动播放');
                    try { player.play(); } catch (e) {}
                    document.removeEventListener('click', unlock);
                    document.removeEventListener('keydown', unlock);
                };
                document.addEventListener('click', unlock);
                document.addEventListener('keydown', unlock);
            } else if (v) {
                console.log('[倍速助手] 自动播放成功');
            }
        }, 1000);
    }

    function showUnlockHint() {
        if (document.getElementById('sdcme-unlock-hint')) return;
        const hint = document.createElement('div');
        hint.id = 'sdcme-unlock-hint';
        hint.style.cssText = `
            position: fixed; top: 20px; left: 50%; transform: translateX(-50%);
            background: rgba(59, 130, 246, 0.95); color: #fff; padding: 10px 20px;
            border-radius: 8px; font-size: 14px; z-index: 1000000;
            box-shadow: 0 4px 12px rgba(0,0,0,0.3);
        `;
        hint.textContent = '👆 点击页面任意位置，解锁自动播放（仅首次需要）';
        document.body.appendChild(hint);
        setTimeout(() => hint.remove(), 5000);
    }

    function updateNextBtn(text) {
        if (nextBtn) nextBtn.textContent = text;
    }

    // ---------- 视频播完检测 ----------
    function checkVideoEnd(video) {
        if (!video) return;
        const remaining = video.duration - video.currentTime;
        // 剩余时间小于阈值，或者视频已经ended，都触发
        if ((remaining < END_THRESHOLD || video.ended) && !window.__alreadyAutoPlayNext) {
            window.__alreadyAutoPlayNext = true;  // 防止重复触发
            console.log('[倍速助手] 视频接近结尾，剩余', remaining.toFixed(1), '秒');
            autoPlayNext(video);
        }
    }

    // ---------- 创建控制面板 ----------
    function createPanel() {
        if (panel) return;

        panel = document.createElement('div');
        panel.style.cssText = `
            position: fixed;
            right: 20px;
            bottom: 90px;
            z-index: 999999;
            background: rgba(15, 23, 42, 0.92);
            border-radius: 12px;
            padding: 12px 14px;
            color: #f1f5f9;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            font-size: 13px;
            box-shadow: 0 8px 32px rgba(0,0,0,0.4);
            min-width: 160px;
            user-select: none;
            backdrop-filter: blur(12px);
            border: 1px solid rgba(255,255,255,0.08);
        `;

        // 标题栏
        const header = document.createElement('div');
        header.style.cssText = 'display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; padding-bottom: 8px; border-bottom: 1px solid rgba(255,255,255,0.1);';

        const title = document.createElement('span');
        title.style.cssText = 'font-weight: 600; font-size: 12px; opacity: 0.8;';
        title.textContent = '倍速控制';

        speedLabel = document.createElement('span');
        speedLabel.style.cssText = `
            font-weight: 700;
            font-size: 18px;
            color: #4ade80;
            background: rgba(74, 222, 128, 0.12);
            padding: 2px 10px;
            border-radius: 6px;
            min-width: 48px;
            text-align: center;
        `;
        speedLabel.textContent = currentSpeed + 'x';

        header.appendChild(title);
        header.appendChild(speedLabel);
        panel.appendChild(header);

        // 倍速按钮网格
        const btnGrid = document.createElement('div');
        btnGrid.style.cssText = 'display: grid; grid-template-columns: repeat(5, 1fr); gap: 4px; margin-bottom: 10px;';

        SPEED_OPTIONS.forEach(speed => {
            const btn = document.createElement('button');
            btn.textContent = speed + 'x';
            const isActive = Math.abs(speed - currentSpeed) < 0.01;
            btn.style.cssText = `
                padding: 5px 2px;
                border: none;
                border-radius: 5px;
                background: ${isActive ? '#4ade80' : 'rgba(255,255,255,0.1)'};
                color: ${isActive ? '#0f172a' : '#e2e8f0'};
                cursor: pointer;
                font-size: 11px;
                font-weight: 600;
                transition: all 0.15s;
            `;
            btn.onclick = () => setSpeed(speed);
            btnGrid.appendChild(btn);
        });
        panel.appendChild(btnGrid);

        // 自定义倍速输入
        const customRow = document.createElement('div');
        customRow.style.cssText = 'display: flex; gap: 6px; align-items: center; margin-bottom: 8px;';

        speedInput = document.createElement('input');
        speedInput.type = 'number';
        speedInput.min = '0.1';
        speedInput.max = MAX_SPEED;
        speedInput.value = currentSpeed;
        speedInput.style.cssText = `
            flex: 1; padding: 5px 8px; border: 1px solid rgba(255,255,255,0.15);
            border-radius: 6px; background: rgba(255,255,255,0.08); color: #fff;
            font-size: 12px; outline: none;
        `;

        const applyBtn = document.createElement('button');
        applyBtn.textContent = '设置';
        applyBtn.style.cssText = `
            padding: 5px 12px; border: none; border-radius: 6px;
            background: #3b82f6; color: #fff; cursor: pointer; font-size: 11px; font-weight: 600;
        `;
        applyBtn.onclick = () => {
            const val = parseFloat(speedInput.value);
            if (!isNaN(val) && val >= 0.1 && val <= MAX_SPEED) setSpeed(val);
        };

        customRow.appendChild(speedInput);
        customRow.appendChild(applyBtn);
        panel.appendChild(customRow);

        // 下一个视频提示（仅播放页）
        if (isPlayPage) {
            nextBtn = document.createElement('div');
            nextBtn.style.cssText = `
                padding: 6px 8px; border-radius: 6px; background: rgba(34, 197, 94, 0.15);
                color: #4ade80; font-size: 11px; text-align: center; margin-bottom: 6px;
            `;
            nextBtn.textContent = '自动连播已开启';
            panel.appendChild(nextBtn);
        }

        // 提示
        const hint = document.createElement('div');
        hint.style.cssText = 'font-size: 10px; opacity: 0.5; text-align: center;';
        hint.textContent = `[ 减速  ] 加速 | 反作弊已绕过`;
        panel.appendChild(hint);

        document.body.appendChild(panel);
    }

    // ---------- 设置倍速 ----------
    function setSpeed(speed) {
        currentSpeed = speed;
        if (speedLabel) speedLabel.textContent = speed + 'x';
        if (speedInput) speedInput.value = speed;

        if (panel) {
            const buttons = panel.querySelectorAll('button');
            buttons.forEach(btn => {
                const text = btn.textContent;
                if (text.endsWith('x') && !isNaN(parseFloat(text))) {
                    const btnSpeed = parseFloat(text);
                    btn.style.background = Math.abs(btnSpeed - speed) < 0.01 ? '#4ade80' : 'rgba(255,255,255,0.1)';
                    btn.style.color = Math.abs(btnSpeed - speed) < 0.01 ? '#0f172a' : '#e2e8f0';
                }
            });
        }
        applySpeed(speed);
    }

    // ---------- 快捷键 ----------
    document.addEventListener('keydown', e => {
        const tag = (e.target.tagName || '').toLowerCase();
        if (tag === 'input' || tag === 'textarea') return;
        if (e.key === HOTKEY_DEC) { e.preventDefault(); setSpeed(SPEED_OPTIONS[Math.max(0, SPEED_OPTIONS.findIndex(s => s >= currentSpeed) - 1)]); }
        if (e.key === HOTKEY_INC) { e.preventDefault(); setSpeed(SPEED_OPTIONS[Math.min(SPEED_OPTIONS.length - 1, SPEED_OPTIONS.findIndex(s => s > currentSpeed) || 0)]); }
    });

    // ---------- 播放事件监听 ----------
    document.addEventListener('play', () => { setTimeout(() => applySpeed(currentSpeed), 200); }, true);
    document.addEventListener('ended', () => {
        console.log('[倍速助手] 视频ended事件触发');
        const video = document.querySelector('video');
        if (video) checkVideoEnd(video);
    }, true);

    // ---------- 播放页：定时检查是否播完 ----------
    function startEndCheck() {
        setInterval(() => {
            const video = document.querySelector('video');
            if (video) checkVideoEnd(video);  // 不管暂停还是播放，都检查
        }, 1000);
    }

    // ---------- 初始化 ----------
    function init() {
        createPanel();

        // 课程页：等视频列表加载出来后再抓取
        if (isCoursePage) {
            let retries = 0;
            const scrapeTimer = setInterval(() => {
                const result = scrapeCourseList();
                if (result) {
                    console.log('[倍速助手] 课程页抓取完成:', result.videos.length, '个视频');
                    clearInterval(scrapeTimer);
                }
                retries++;
                if (retries > 15) {
                    console.log('[倍速助手] 课程页抓取超时，未找到视频列表');
                    clearInterval(scrapeTimer);
                }
            }, 1000);
        }

        // 播放页：等播放器加载
        if (isPlayPage) {
            let retries = 0;
            const initTimer = setInterval(() => {
                if (getPlayer()) {
                    bypassAntiCheat();  // 尝试hook
                    startHookWatcher(); // 启动监控，每2秒检查hook是否被覆盖
                    applySpeed(currentSpeed);
                    startEndCheck();
                    clearInterval(initTimer);
                    // 打印初始状态
                    setTimeout(logPlaybackStatus, 1000);
                    // 自动播放
                    setTimeout(autoPlayVideo, 500);
                }
                retries++;
                if (retries > 30) clearInterval(initTimer);
            }, 1000);
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
