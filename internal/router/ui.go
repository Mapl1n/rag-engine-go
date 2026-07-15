package router

import "github.com/gin-gonic/gin"

func serveUI(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(200, uiHTML)
}

const uiHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>📚 RAG 文档问答引擎</title>
<style>
:root{--bg:#0f172a;--card:#1e293b;--border:#334155;--text:#e2e8f0;--muted:#94a3b8;--accent:#6366f1;--green:#22c55e;--red:#ef4444}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:var(--bg);color:var(--text);min-height:100vh}
.header{background:var(--card);border-bottom:1px solid var(--border);padding:12px 24px;display:flex;justify-content:space-between;align-items:center}
.header h1{font-size:18px;display:flex;align-items:center;gap:8px}
.status{font-size:12px;display:flex;gap:12px}
.status .dot{display:inline-block;width:8px;height:8px;border-radius:50%}
.dot-on{background:var(--green)}.dot-off{background:var(--red)}
.container{max-width:900px;margin:0 auto;padding:20px;display:grid;grid-template-columns:280px 1fr;gap:20px}
@media(max-width:700px){.container{grid-template-columns:1fr}}
.card{background:var(--card);border:1px solid var(--border);border-radius:10px;padding:16px;margin-bottom:16px}
.card h3{font-size:14px;margin-bottom:12px;color:var(--accent)}
.btn{padding:8px 16px;border-radius:6px;border:none;cursor:pointer;font-size:13px;font-weight:500}
.btn-primary{background:var(--accent);color:#fff;width:100%}
.btn-primary:hover{opacity:.9}
.btn-sm{padding:4px 10px;font-size:12px}
input,textarea{width:100%;background:var(--bg);border:1px solid var(--border);color:var(--text);padding:8px 12px;border-radius:6px;font-size:13px;outline:none;resize:none}
input:focus,textarea:focus{border-color:var(--accent)}
textarea{min-height:80px}
.upload-zone{border:2px dashed var(--border);border-radius:10px;padding:30px;text-align:center;cursor:pointer;transition:all .2s;margin-bottom:12px}
.upload-zone:hover{border-color:var(--accent);background:rgba(99,102,241,0.05)}
.upload-zone p{font-size:13px;color:var(--muted)}
.result-item{padding:12px;border-left:3px solid var(--accent);margin-bottom:10px;background:var(--bg);border-radius:0 6px 6px 0}
.result-item .meta{font-size:11px;color:var(--muted);margin-bottom:4px}
.result-item .text{font-size:13px;line-height:1.6}
.result-item .hl{background:rgba(99,102,241,0.3);padding:1px 2px;border-radius:2px}
.answer-box{padding:16px;background:var(--bg);border-radius:8px;margin-bottom:12px;font-size:14px;line-height:1.7;white-space:pre-wrap}
.streaming{cursor:pointer}
.doc-item{display:flex;justify-content:space-between;align-items:center;padding:8px 10px;border:1px solid var(--border);border-radius:6px;margin-bottom:6px;font-size:12px}
.doc-item:hover{background:rgba(99,102,241,0.05)}
.spinner{display:inline-block;width:16px;height:16px;border:2px solid var(--border);border-top-color:var(--accent);border-radius:50%;animation:spin .6s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}
#toast{position:fixed;top:20px;right:20px;z-index:9999}
.toast-msg{padding:10px 18px;border-radius:8px;font-size:12px;margin-bottom:8px}
.toast-success{background:#065f46;color:#6ee7b7}
.toast-error{background:#7f1d1d;color:#fca5a5}
.score-bar{display:inline-block;height:4px;background:var(--accent);border-radius:2px;margin-right:4px}
</style>
</head>
<body>

<div class="header">
  <h1>📚 RAG 文档问答引擎</h1>
  <div id="statusBar" class="status"><span class="spinner"></span> 检测服务...</div>
</div>

<div id="toast"></div>

<div class="container">
  <!-- Left: Upload -->
  <div>
    <div class="card">
      <h3>📤 上传文档</h3>
      <div class="upload-zone" id="dropZone" onclick="document.getElementById('fileInput').click()">
        <p style="font-size:24px;margin-bottom:8px">📄</p>
        <p>点击或拖拽上传 PDF/DOCX</p>
        <p style="font-size:11px;margin-top:4px">自动解析 → 分块 → 向量化 → 索引</p>
      </div>
      <input type="file" id="fileInput" accept=".pdf,.docx,.doc,.txt,.html" style="display:none" onchange="uploadFile()">
      <div id="uploadStatus" style="font-size:12px;color:var(--muted);margin-top:8px"></div>
    </div>

    <div class="card">
      <h3>📂 已上传文档</h3>
      <div id="docList" style="max-height:300px;overflow-y:auto"></div>
    </div>
  </div>

  <!-- Right: Search -->
  <div>
    <div class="card">
      <h3>🔍 语义检索 + 问答</h3>
      <textarea id="queryInput" placeholder="输入你的问题，例如：这份合同的有效期到什么时候？" onkeydown="if(event.key==='Enter'&&!event.shiftKey){event.preventDefault();search()}"></textarea>
      <div style="display:flex;gap:8px;margin-top:8px">
        <select id="docFilter" style="flex:1;font-size:12px;padding:6px;background:var(--bg);border:1px solid var(--border);color:var(--text);border-radius:6px">
          <option value="">所有文档</option>
        </select>
        <button class="btn btn-primary" style="width:auto" onclick="search()">🔍 检索</button>
        <button class="btn btn-primary" style="width:auto;background:var(--green)" onclick="qaStream()" id="qaBtn">🤖 AI 问答</button>
      </div>
    </div>

    <div id="resultsArea">
      <p style="text-align:center;color:var(--muted);margin:40px 0">输入问题开始搜索</p>
    </div>
  </div>
</div>

<script>
const API='/api';
let status={tika:false,ollama:false,qa:false};

fetch(API+'/health').then(r=>r.json()).then(d=>{
  status=d;
  document.getElementById('statusBar').innerHTML=
    '<span class="dot '+(d.tika?'dot-on':'dot-off')+'"></span> Tika '+
    '<span class="dot '+(d.ollama?'dot-on':'dot-off')+'"></span> Ollama '+
    '<span class="dot '+(d.qa?'dot-on':'dot-off')+'"></span> LLM';
  if(!d.qa) document.getElementById('qaBtn').textContent='🤖 仅检索';
  loadDocs();
}).catch(()=>{
  document.getElementById('statusBar').innerHTML='<span class="dot dot-off"></span> 服务离线';
});

const dropZone=document.getElementById('dropZone');
dropZone.ondragover=e=>{e.preventDefault();dropZone.style.borderColor='var(--accent)'};
dropZone.ondragleave=()=>dropZone.style.borderColor='var(--border)';
dropZone.ondrop=e=>{e.preventDefault();dropZone.style.borderColor='var(--border)';
  const f=e.dataTransfer.files[0];if(f)doUpload(f)};

function uploadFile(){const f=document.getElementById('fileInput').files[0];if(f)doUpload(f)}
async function doUpload(file){
  document.getElementById('uploadStatus').innerHTML='<span class="spinner"></span> 解析中...';
  const fd=new FormData();fd.append('file',file);
  try{
    const r=await fetch(API+'/documents/upload',{method:'POST',body:fd});
    const d=await r.json();
    if(d.code===0){
      toast('✅ '+d.data.filename+' · '+d.data.chunk_count+' chunks','success');
      loadDocs();
    }else{toast(d.message,'error')}
  }catch(e){toast(e.message,'error')}
  document.getElementById('uploadStatus').innerHTML='';
}

async function loadDocs(){
  try{
    const r=await fetch(API+'/documents');const docs=await r.json();
    const list=document.getElementById('docList');
    const sel=document.getElementById('docFilter');
    if(!docs.data||docs.data.length===0){list.innerHTML='<p style="font-size:12px;color:var(--muted)">暂无文档</p>';sel.innerHTML='<option value="">所有文档</option>';return}
    list.innerHTML=docs.data.map(d=>'<div class="doc-item"><span>📄 '+d.filename+'<br><span style="color:var(--muted)">'+d.chunk_count+' chunks · '+d.created_at+'</span></span></div>').join('');
    sel.innerHTML='<option value="">所有文档</option>'+docs.data.map(d=>'<option value="'+d.id+'">'+d.filename+'</option>').join('');
  }catch(e){}
}

async function search(){
  const q=document.getElementById('queryInput').value.trim();
  if(!q)return;
  const area=document.getElementById('resultsArea');
  area.innerHTML='<p style="text-align:center"><span class="spinner"></span> 语义检索中...</p>';
  try{
    const r=await fetch(API+'/search',{method:'POST',headers:{'Content-Type':'application/json'},
      body:JSON.stringify({query:q,top_k:5,doc_id:document.getElementById('docFilter').value})});
    const d=await r.json();
    if(!d.data||!d.data.results||d.data.results.length===0){
      area.innerHTML='<p style="text-align:center;color:var(--muted);margin:40px">未找到相关内容<br><span style="font-size:12px">试试换个关键词或上传更多文档</span></p>';return
    }
    area.innerHTML='<h4 style="margin-bottom:12px">找到 '+d.data.total+' 条结果 ('+d.data.took_ms+'ms)</h4>'+
      d.data.results.map(r=>'<div class="result-item"><div class="meta">📄 '+r.doc_name+' · 相关度 '+(r.score*100).toFixed(0)+'%</div><div class="text">'+highlight(r.text,q)+'</div></div>').join('');
  }catch(e){area.innerHTML='<p style="color:var(--red)">搜索失败: '+e.message+'</p>'}
}

function highlight(text,query){
  const words=query.split(/\s+/).filter(w=>w.length>1);
  let t=text.substring(0,500);
  words.forEach(w=>{t=t.replace(new RegExp('('+w+')','gi'),'<span class="hl">$1</span>')});
  return t;
}

async function qaStream(){
  const q=document.getElementById('queryInput').value.trim();
  if(!q)return;
  const area=document.getElementById('resultsArea');
  area.innerHTML='<div class="answer-box" id="streamBox"><span class="spinner"></span> 思考中...</div><div id="sourceBox"></div>';
  try{
    const r=await fetch(API+'/qa/stream',{method:'POST',headers:{'Content-Type':'application/json'},
      body:JSON.stringify({question:q,top_k:5,doc_id:document.getElementById('docFilter').value})});
    const reader=r.body.getReader();const decoder=new TextDecoder();
    const box=document.getElementById('streamBox');box.innerHTML='';
    while(true){
      const{value,done}=await reader.read();if(done)break;
      const lines=decoder.decode(value).split('\n');
      for(const line of lines){
        if(line.startsWith('data: ')){
          const data=line.substring(6);
          if(data==='[DONE]'){box.innerHTML+='<br><span style="color:var(--muted);font-size:11px">✅ 回答完成</span>';return}
          try{box.innerHTML+=data.replace(/\\n/g,'<br>')}catch(e){box.innerHTML+=data}
        }
      }
    }
  }catch(e){area.innerHTML='<p style="color:var(--red)">QA失败: '+e.message+'</p>'}
}

function toast(msg,type){
  const el=document.getElementById('toast');
  const d=document.createElement('div');
  d.className='toast-msg toast-'+type;d.textContent=msg;
  el.appendChild(d);setTimeout(()=>d.remove(),3000);
}
</script>
</body></html>`
