---
name: local-file-generation
description: Use when asked to create docx, pptx, xlsx, or pdf files.
---
# File Generation Skill

Generate `.pptx`, `.docx`, `.xlsx`, and `.pdf` files entirely client-side using browser-based libraries.

---

## 1. XLSX Generation with Pyodide + openpyxl

Use **Pyodide** to run Python in the browser and `openpyxl` to build styled workbooks with native Excel charts.

### CDN
```html
<script src="https://cdn.jsdelivr.net/pyodide/v0.27.2/full/pyodide.js"></script>
```

### Critical: Script Load Order
The Pyodide CDN must load **before** any script that calls `loadPyodide()`.

```html
<!-- ✅ CORRECT: Pyodide loads first -->
<script src="https://cdn.jsdelivr.net/pyodide/v0.27.2/full/pyodide.js"></script>
<script>
  async function createXlsx() {
    const pyodide = await loadPyodide();  // now defined
    // ...
  }
</script>

<!-- ❌ WRONG: calling loadPyodide before the CDN loads -->
<script>
  const pyodide = await loadPyodide();  // ReferenceError
</script>
<script src="https://cdn.jsdelivr.net/pyodide/v0.27.2/full/pyodide.js"></script>
```

### Critical: Never Auto-Generate or Auto-Download
- Do **not** call generation on page load.
- Do **not** trigger `a.click()` automatically.
- Always present a clickable file card or button and let the user initiate generation/download.

### Basic Pattern
```javascript
async function createXlsx() {
  const pyodide = await loadPyodide();
  await pyodide.loadPackage('micropip');

  await pyodide.runPythonAsync(`
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
  const blob = new Blob([bytes], {
    type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
  });
  const url = URL.createObjectURL(blob);

  const a = document.createElement('a');
  a.href = url;
  a.download = 'report.xlsx';
  a.click();
  URL.revokeObjectURL(url);
}
```

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

### File Card
Always use a clickable card/button. Never auto-download.

```html
<button id="generate-btn" style="width:100%; padding:1rem; border:1px solid var(--border); border-radius:0.75rem; background:var(--background); color:var(--foreground); cursor:pointer;">
  Generate monthly_report.xlsx
</button>
```

```javascript
document.getElementById('generate-btn').addEventListener('click', async () => {
  // run generation and download here
});
```

### Best Practice: Preview First
For complex layouts, build a frontend preview first, agree on it with the user, then replicate the exact cell positions and dimensions in openpyxl.

---

## 2. PPTX Generation with PptxGenJS

```html
<script src="https://cdn.jsdelivr.net/gh/gitbrent/pptxgenjs/dist/pptxgen.bundle.js"></script>
```

```javascript
const ppt = new PptxGenJS();
ppt.layout = 'LAYOUT_16x9';

const slide = ppt.addSlide();
slide.background = { color: '111827' };
slide.addText('Hello', { x: 1, y: 2, w: '80%', fontSize: 36, align: 'center' });
slide.addChart(ppt.charts.BAR, [...], { x: 0.5, y: 1.5, w: 9, h: 5 });
slide.addTable([['A','B'], ['1','2']], { x: 0.5, y: 1, w: 9 });

ppt.writeFile({ fileName: 'slides.pptx' });
```

---

## 3. DOCX Generation with docx.js

```html
<script src="https://cdn.jsdelivr.net/npm/docx/dist/index.iife.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/file-saver@2.0.5/dist/FileSaver.min.js"></script>
```

```javascript
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

Packer.toBlob(doc).then(blob => saveAs(blob, 'doc.docx'));
```

---

## 4. PDF Generation with jsPDF

Generate PDFs client-side with jsPDF.

```html
<script src="https://cdn.jsdelivr.net/npm/jspdf@2.5.1/dist/jspdf.umd.min.js"></script>
```

```javascript
function createPdf() {
  const { jsPDF } = window.jspdf;
  const pdf = new jsPDF("p", "pt", "a4");

  pdf.setFontSize(18);
  pdf.text("Document title", 48, 60);

  pdf.setFontSize(11);
  pdf.text("Document content goes here.", 48, 90);

  const blob = pdf.output("blob");
  const url = URL.createObjectURL(blob);

  const link = document.createElement("a");
  link.href = url;
  link.download = "document.pdf";
  link.textContent = "Download PDF";
  document.body.appendChild(link);
}
```

Rules:

- Generate only after a user clicks a button.
- Track the current vertical position when adding content.
- Add new pages when content reaches the page bottom.
- Use a clickable download link or button.
- Never auto-generate or auto-download files.

### Mermaid Diagrams in PDFs

Mermaid diagrams can be rendered as SVG and embedded directly into jsPDF.

```html
<script src="https://cdn.jsdelivr.net/npm/mermaid@10.9.1/dist/mermaid.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/svg2pdf.js@2.7.0/dist/svg2pdf.umd.min.js"></script>
```

```javascript
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
- For widget styling, the outer widget root must have `width:100%; padding:0; margin:0; border:none;`. Internal elements handle their own spacing.
- Always provide a manual download trigger — never auto-generate or auto-download.
- If a widget errors or the user requests an update, rewrite the widget. Do not ask the user to fix or update it themselves.