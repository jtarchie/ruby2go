// Prints the landing page benchmark chart (site/index.html, from "About 3× faster" up to "Why a werewolf?"): node scripts/landing-bench.mjs
// ponytail: rows copied by hand from BENCHMARKS.md (2026-10-25); parse that table if it gets rerun often
const rows = [
  ['fib(32)', .27, .092, .017], ['Hash counting', .11, .096, .015], ['Array map/select/sum', .15, .14, .021],
  ['string build/split', .11, .11, .030], ['Math.sin/cos', .086, .072, .009], ['classes, polymorphic calls', .26, .17, .036],
  ['binary trees', .082, .068, .010], ['splay tree', .21, .12, .013], ['blocks, procs, enumerators', .36, .34, .064],
  ['raise/rescue, catch/throw', .28, .25, .12], ['regexp, StringScanner', .76, .74, .21], ['sort_by/group_by/tally, Set', .55, .53, .12],
  ['format, StringIO', .49, .44, .24], ['JSON generate + parse', .14, .15, .12], ['Bignum', .21, .21, .059],
  ['pairing_heap gem', .68, .34, .21], ['liquid gem', .38, .30, .17], ['kramdown gem', .45, .55, .41],
].map(([n, mri, yjit, rg]) => ({ n, mri, yjit, rg, sr: mri / rg, sy: mri / yjit })).sort((a, b) => b.sr - a.sr);
const max = 18, pct = v => (v / max * 100).toFixed(2), x = v => v >= 10 ? v.toFixed(0) : v.toFixed(1);
const t = s => `${s < 1 ? (s * 1000).toFixed(0) + ' ms' : s + ' s'}`;
let h = rows.map(r => `      <div class="brow"><span class="bname">${r.n}</span><span class="btrack" title="${r.n}: MRI ${t(r.mri)}, YJIT ${t(r.yjit)}, rugaroo ${t(r.rg)}"><span class="l1"><span class="bar rg" style="width:${pct(r.sr)}%"></span><span class="bval">${x(r.sr)}×</span></span><span class="bar yj" style="width:${pct(r.sy)}%"></span></span></div>`).join('\n');
let tbl = rows.map(r => `<tr><td>${r.n}</td><td>${t(r.mri)}</td><td>${t(r.yjit)}</td><td>${t(r.rg)}</td><td>${x(r.sr)}×</td></tr>`).join('');
console.log(`  <h2>About 3× faster than MRI</h2>
  <p class="sub">Speedup over plain MRI (1×) on the <a href="https://github.com/jtarchie/rugaroo/tree/main/benchmarks">benchmark workloads</a>, against MRI with YJIT. All 18 together take 1.87 s compiled and 5.58 s on MRI. Median of 9 runs on one Apple-silicon machine.</p>
  <figure class="bench" aria-labelledby="bench-cap">
    <figcaption id="bench-cap" class="legend"><span><i class="sw rg"></i>rugaroo</span><span><i class="sw yj"></i>MRI + YJIT</span><span><i class="sw base"></i>MRI = 1×</span></figcaption>
    <div class="bchart" style="--one:${pct(1)}%">
${h}
      <div class="brow axis" aria-hidden="true"><span class="bname"></span><span class="btrack">${[1, 5, 10, 15].map(v => `<span style="left:${pct(v)}%">${v}×</span>`).join('')}</span></div>
    </div>
    <details><summary>Table</summary><table><thead><tr><th>workload</th><th>MRI</th><th>MRI+YJIT</th><th>rugaroo</th><th>speedup</th></tr></thead><tbody>${tbl}</tbody></table></details>
  </figure>
  <p class="sub out">Where it trails, and how it compares with spinel, Crystal and hand-written Go: <a href="https://github.com/jtarchie/rugaroo/blob/main/BENCHMARKS.md">BENCHMARKS.md</a>.</p>
`);
