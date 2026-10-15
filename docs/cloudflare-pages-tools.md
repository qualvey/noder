# Cloudflare Pages ÕÀÑÕàÀþëê

µ£¼Õêåµö»Õ░åÕø¥þëçµ░┤Õì░ÒÇüÞèéþé╣Úàìþ¢«Þ¢¼µìóÒÇüõ║îþ╗┤þáüõ©ëõ©¬ÕÀÑÕàÀµè¢µêÉþï¼þ½ïþÜäþ║»Õëìþ½»ÚíÁÚØóÒÇéµëÇµ£ëÕø¥þëçÕÆîÚàìþ¢«õ╗àÕ£¿µÁÅÞºêÕÖ¿ÕåàÕñäþÉå´╝îõ©ìÞ»Àµ▒é Noder APIÒÇé

Þèéþé╣Þ¢¼µìóÕÀÑÕàÀµö»µîüÕñÜµØí VLESS Õêåõ║½Úô¥µÄÑ´╝Üµ£ëµòêÚô¥µÄÑõ╝Üþ╗ºþ╗¡ÕñäþÉå´╝îÚçìÕñì tag Þç¬Õè¿Þ┐¢Õèá `-2`ÒÇü`-3`´╝îÚöÖÞ»»ÞíîÕìòþï¼µÅÉþñ║´╝îÕ╣ÂÕÅ»þöƒµêÉÕ«îµò┤þÜä sing-box Õ«óµêÀþ½» JSON Úàìþ¢«µêûõ©ïÞ¢¢µûçõ╗ÂÒÇé

## Pages Úàìþ¢«

Õ£¿ Cloudflare Pages õ©¡õ¢┐þö¿õ╗Ñõ©ïÞ«¥þ¢«´╝Ü

- **Root directory**´╝Ü`web`
- **Build command**´╝Ü`pnpm install --frozen-lockfile && pnpm run build:pages`
- **Build output directory**´╝Ü`pages-dist`
- **Node.js / pnpm**´╝Üõ¢┐þö¿Úí╣þø«ÚöüÕ«Üµûçõ╗ÂÕ»╣Õ║öþÜä pnpm þëêµ£¼

õ╣ƒÕÅ»õ╗ÑÕ£¿õ╗ôÕ║ôµá╣þø«Õ¢òµëºÞíî´╝Ü

```bash
pnpm --dir web install --frozen-lockfile
pnpm --dir web exec vue-tsc --noEmit
pnpm --dir web exec vite build --config vite.pages.config.ts
```

þöƒµêÉþø«Õ¢òõ©║ `web/pages-dist`ÒÇé

## õ©ÄÕ«îµò┤ Noder þÜäÕà│þ│╗

Þ»ÑÕàÑÕÅúõ¢┐þö¿ `tools/index.html` ÕÆî `ToolsApp.vue`´╝îõ©ìõ╝ÜÕèáÞ¢¢þ«íþÉåÕæÿþÖ╗Õ¢òÒÇüSQLite µêû Go APIÒÇéÕÄƒµ£ë `web/index.html` ÕÆîÕÉÄþ½»ÚØÖµÇüÞÁäµ║Éµ×äÕ╗║µÁüþ¿ïõ┐Øµîüõ©ìÕÅÿÒÇé
