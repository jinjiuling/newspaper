var dark = function() { document.documentElement.setAttribute("color-mode", "dark"); localStorage.setItem("color-mode", "dark"); return; }
var light = function() { document.documentElement.setAttribute("color-mode", "light"); localStorage.setItem("color-mode", "light"); return; }
var microfiche = function() { document.documentElement.setAttribute("color-mode", "microfiche"); localStorage.setItem("color-mode", "microfiche"); return; }
var dracula = function() { document.documentElement.setAttribute("color-mode", "dracula"); localStorage.setItem("color-mode", "dracula"); return; }

var sans = function() { document.documentElement.setAttribute("font-mode", "sans"); localStorage.setItem("font-mode", "sans"); }
var serif = function() { document.documentElement.setAttribute("font-mode", "serif"); localStorage.setItem("font-mode", "serif"); }

var big = function() { document.documentElement.setAttribute("font-size", "big"); localStorage.setItem("font-size", "big"); }
var regular = function() { document.documentElement.setAttribute("font-size", "regular"); localStorage.setItem("font-size", "regular"); }

/* === 页面加载时自动恢复用户设置 ===
document.addEventListener("DOMContentLoaded", function() {
    // 恢复排版模式
    const savedFont = localStorage.getItem("font-mode");
    if (savedFont) {
        document.documentElement.setAttribute("font-mode", savedFont);
    }
    
    // 恢复字号
    const savedSize = localStorage.getItem("font-size");
    if (savedSize) {
        document.documentElement.setAttribute("font-size", savedSize);
    }
});*/
