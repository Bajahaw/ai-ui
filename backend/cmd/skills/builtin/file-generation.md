---
name: file-generation
description: Use when asked to create docx, pptx, xlsx, or pdf files.
---
# File Generation Skill

Generate `.pptx`, `.docx`, `.xlsx`, and `.pdf` files in `browser_sandbox` using browser-based libraries. Call `await sandbox.writeFile(name, data, mime)` then `sandbox.done()`. Do not use widgets or auto-download for the file itself.

### Sandbox rules (read first)
- The sandbox runs **JavaScript only**, in an async context: top-level `await` and `return` work. There is no visible page and no HTML mode. Never build an HTML document or `<script>` tags as a string.
- Load libraries with `await sandbox.loadScript(url)`. It resolves after the script has executed, so the library global is ready on the next line. It rejects immediately on a wrong URL, version, or path, and on any host outside the CSP allowlist (`cdn.jsdelivr.net`, `cdnjs.cloudflare.com`), so a bad URL fails fast instead of hanging until `sandbox timed out`. On rejection, fix the URL; do not retry the same one.
- Use the exact CDN URLs below verbatim. Never invent a version or path.
- Whatever the code `return`s is JSON-serialized and reported back, together with `console.log` output. Use `return sandbox.listFiles()` to see mounted files, and `console.log` to inspect intermediate values.
- **Nested backticks.** When embedding Python (or any other source) in a template literal, wrap it in `String.raw` and make sure the embedded source contains no backtick characters. An unescaped inner backtick ends the outer string and everything after it is parsed as JavaScript (typical symptom: `Cannot use import statement outside a module`).
- Always emit files with `sandbox.writeFile` and finish with `sandbox.done()`. Bare JS is awaited automatically; `sandbox.done(error)` reports a failure explicitly.

---

## 1. XLSX Generation with Pyodide + openpyxl

Use **Pyodide** to run Python in the browser and `openpyxl` to build styled workbooks with native Excel charts.

### CDN
```javascript
await sandbox.loadScript('https://cdn.jsdelivr.net/pyodide/v0.27.2/full/pyodide.js');
```

### Critical: Load Order
`loadPyodide()` is only defined after `sandbox.loadScript(...)` has resolved. Always `await` the load before calling it.

```javascript
// ✅ CORRECT: Pyodide loads first
await sandbox.loadScript('https://cdn.jsdelivr.net/pyodide/v0.27.2/full/pyodide.js');
const pyodide = await loadPyodide();  // now defined

// ❌ WRONG: calling loadPyodide before the CDN loads
const pyodide = await loadPyodide();  // ReferenceError
await sandbox.loadScript('https://cdn.jsdelivr.net/pyodide/v0.27.2/full/pyodide.js');
```

### Critical: Write the file through the sandbox
- Do **not** trigger `a.click()` or auto-download.
- Call `await sandbox.writeFile('report.xlsx', bytes, mime)` then `sandbox.done()`.
- Bare JS is awaited automatically; still finish with `sandbox.done()`.

### Basic Pattern
Complete, runnable as-is. Note `String.raw` around the Python and the absence of backticks inside it.

```javascript
await sandbox.loadScript('https://cdn.jsdelivr.net/pyodide/v0.27.2/full/pyodide.js');
const pyodide = await loadPyodide();
await pyodide.loadPackage('micropip');

await pyodide.runPythonAsync(String.raw`
import micropip
await micropip.install('openpyxl')
from openpyxl import Workbook

wb = Workbook()
ws = wb.active
ws.title = 'Sales'

for row in [['Product', 'Sales'], ['A', 100], ['B', 200], ['C', 150]]:
    ws.append(row)

wb.save('report.xlsx')
`);

const bytes = pyodide.FS.readFile('report.xlsx');
await sandbox.writeFile('report.xlsx', bytes, 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet');
sandbox.done();
```

### Formulas show blank until recalculated
openpyxl writes formulas without cached results, so previewers and viewers that do not recalculate show empty cells. When the user needs to see numbers immediately, write the computed values into the cells (and put the formulas in a separate column or sheet), or at minimum set `wb.calculation = CalcProperties(fullCalcOnLoad=True)` from `openpyxl.workbook.properties` so Excel recalculates on open.

### Adding Charts

#### Default Chart Size
By default, openpyxl charts are **15 cm × 7.5 cm** and anchor at E15. Always set explicit `width`, `height`, and anchor cells.

```python
from openpyxl.chart import BarChart, PieChart, LineChart, Reference

# Bar chart
bar = BarChart()
bar.title = 'Revenue & Profit'
bar.add_data(Reference(ws, min_col=2, min_row=1, max_col=3, max_row=7), titles_from_data=True)
bar.set_categories(Reference(ws, min_col=1, min_row=2, max_row=7))
bar.width = 14
bar.height = 7.5
bar.legend.position = 'b'
bar.legend.overlay = False
ws.add_chart(bar, 'A11')
```

#### Chart Placement Rules
- Charts anchor to the **top-left cell** of their container.
- Two charts at the same anchor cell will overlap.
- Plan your sheet layout like a grid. Give each chart its own anchor with enough vertical/horizontal clearance.
- Adjacent charts should share the **same height** for visual alignment.

```python
# Adjacent row of charts
bar.width, bar.height = 14, 7.5
pie.width, pie.height = 6.5, 7.5
line.width, line.height = 6.5, 7.5

ws.add_chart(bar, 'A11')   # left
ws.add_chart(pie, 'I11')   # middle-right
ws.add_chart(line, 'M11')  # far right
```

#### Line Chart Data Reference
Include the header row in `add_data()` so the series is named correctly and produces **one line**, not one line per category.

```python
line = LineChart()
line.add_data(Reference(ws, min_col=2, min_row=1, max_row=5), titles_from_data=True)
line.set_categories(Reference(ws, min_col=1, min_row=2, max_row=5))
line.legend = None  # optional for single-series
```

#### Pie Chart Labels
Use `DataLabelList` carefully to avoid duplicated text.

```python
from openpyxl.chart.label import DataLabelList

pie = PieChart()
pie.add_data(Reference(ws, min_col=2, min_row=1, max_row=5), titles_from_data=True)
pie.set_categories(Reference(ws, min_col=1, min_row=2, max_row=5))

dl = DataLabelList()
dl.showPercent = True
dl.showCatName = True
dl.showSerName = False
dl.showVal = False
pie.dataLabels = dl
```

### Styling & Layout
- Set column widths explicitly for clean tables.
- Use `Alignment(horizontal='center', vertical='center')`.
- Use `PatternFill` for header backgrounds.
- Merge cells for titles, but write only to the top-left cell.

```python
ws.merge_cells('A1:O1')
ws['A1'] = 'Monthly Performance Report'
ws['A1'].font = Font(size=18, bold=True, color='FFFFFF')
ws['A1'].fill = PatternFill('solid', fgColor='1F2937')
ws['A1'].alignment = Alignment(horizontal='center', vertical='center')
```

After the sandbox returns a file id, send the user a markdown file link: `[monthly_report.xlsx](/data/resources/{file_id}.xlsx)`.

### Best Practice: Preview First
For complex layouts, build a frontend preview first, agree on it with the user, then replicate the exact cell positions and dimensions in openpyxl.

---

## 2. PPTX Generation with PptxGenJS

```javascript
await sandbox.loadScript('https://cdn.jsdelivr.net/gh/gitbrent/pptxgenjs/dist/pptxgen.bundle.js');

const ppt = new PptxGenJS();
ppt.layout = 'LAYOUT_16x9';

const slide = ppt.addSlide();
slide.background = { color: '111827' };
slide.addText('Hello', { x: 1, y: 2, w: '80%', fontSize: 36, align: 'center' });
slide.addChart(ppt.charts.BAR, [...], { x: 0.5, y: 1.5, w: 9, h: 5 });
slide.addTable([['A','B'], ['1','2']], { x: 0.5, y: 1, w: 9 });

const blob = await ppt.write('blob');
await sandbox.writeFile('slides.pptx', blob);
sandbox.done();
```

---

## 3. DOCX Generation with docx.js

```javascript
await sandbox.loadScript('https://cdn.jsdelivr.net/npm/docx/dist/index.iife.min.js');

const { Document, Packer, Paragraph, TextRun, HeadingLevel } = docx;

const doc = new Document({
  sections: [{
    children: [
      new Paragraph({ text: 'Title', heading: HeadingLevel.TITLE }),
      new Paragraph({ text: 'Heading', heading: HeadingLevel.HEADING_1 }),
      new Paragraph({
        children: [
          new TextRun('Normal text. '),
          new TextRun({ text: 'Bold text.', bold: true })
        ]
      })
    ]
  }]
});

const blob = await Packer.toBlob(doc);
await sandbox.writeFile('doc.docx', blob);
sandbox.done();
```

---

## 4. PDF Generation with jsPDF

Generate PDFs client-side with jsPDF.

```javascript
await sandbox.loadScript('https://cdn.jsdelivr.net/npm/jspdf@2.5.1/dist/jspdf.umd.min.js');

const { jsPDF } = window.jspdf;
const pdf = new jsPDF("p", "pt", "a4");

pdf.setFontSize(18);
pdf.text("Document title", 48, 60);

pdf.setFontSize(11);
pdf.text("Document content goes here.", 48, 90);

await sandbox.writeFile("document.pdf", pdf.output("blob"), "application/pdf");
sandbox.done();
```

Rules:

- Track the current vertical position when adding content.
- Add new pages when content reaches the page bottom.

### Mermaid Diagrams in PDFs

Mermaid diagrams can be rendered as SVG and embedded directly into jsPDF.

```javascript
await sandbox.loadScript('https://cdn.jsdelivr.net/npm/jspdf@2.5.1/dist/jspdf.umd.min.js');
await sandbox.loadScript('https://cdn.jsdelivr.net/npm/mermaid@10.9.1/dist/mermaid.min.js');
await sandbox.loadScript('https://cdn.jsdelivr.net/npm/svg2pdf.js@2.7.0/dist/svg2pdf.umd.min.js');

const result = await mermaid.render("diagram" + Date.now(), mermaidCode);

const svg = new DOMParser()
  .parseFromString(result.svg, "image/svg+xml")
  .documentElement;

await pdf.svg(svg, {
  x: 48,
  y: 120,
  width: 495,
  height: 280
});
```

Use `pdf.svg()` for Mermaid diagrams. Avoid canvas, `toDataURL()`, and `window.svg2pdf()`. Mermaid IDs must not contain periods or other invalid CSS-selector characters.

---

## 5. General Rules

- Use Pyodide version `https://cdn.jsdelivr.net/pyodide/v0.27.2/full/pyodide.js` exactly.
- Use MIME type `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` for `.xlsx`.
- Load every library with `await sandbox.loadScript(url)` before using it. Only `cdn.jsdelivr.net` and `cdnjs.cloudflare.com` are allowed.
- Always emit files with `sandbox.writeFile` and finish with `sandbox.done()`.
- If the sandbox errors, read the error and logs, fix the code, and run `browser_sandbox` again. Do not ask the user to fix it.
- To inspect a previously generated file, `sandbox.readFile(name)` returns its bytes (conversation files are auto-mounted); `return` or `console.log` what you find rather than regenerating blindly.
- The sandbox ships disabled and approval-gated: if the tool is unavailable or a run stalls, ask the user to enable it under Settings → Tools and approve the run.
