package router

import "github.com/gin-gonic/gin"

func serveUI(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(200, ragUI)
}

const ragUI = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>RAG 文档问答引擎</title>
<style>
:root{--bg:#0f172a;--card:#1e293b;--border:#334155;--text:#e2e8f0;--muted:#94a3b8;--accent:#6366f1;--green:#22c55e;--red:#ef4444}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Microsoft YaHei',sans-serif;background:var(--bg);color:var(--text);min-height:100vh}
.header{background:var(--card);border-bottom:1px solid var(--border);padding:12px 24px;display:flex;justify-content:space-between;align-items:center}
.header h1{font-size:18px}
.status{font-size:12px;display:flex;gap:12px}
.dot{display:inline-block;width:8px;height:8px;border-radius:50%}
.dot-on{background:var(--green)}.dot-off{background:var(--red)}
.container{max-width:900px;margin:0 auto;padding:20px;display:grid;grid-template-columns:280px 1fr;gap:20px}
@media(max-width:700px){.container{grid-template-columns:1fr}}
.card{background:var(--card);border:1px solid var(--border);border-radius:10px;padding:16px;margin-bottom:16px}
.card h3{font-size:14px;margin-bottom:12px;color:var(--accent)}
.btn{padding:8px 16px;border-radius:6px;border:none;cursor:pointer;font-size:13px;font-weight:500}
.btn-primary{background:var(--accent);color:#fff;width:100%}
.btn-sm{padding:4px 10px;font-size:12px}
.btn-danger{background:var(--red);color:#fff;padding:2px 6px;font-size:10px;border-radius:4px;cursor:pointer;border:none}
input,textarea{width:100%;background:var(--bg);border:1px solid var(--border);color:var(--text);padding:8px 12px;border-radius:6px;font-size:13px;outline:none;resize:none}
textarea{min-height:80px}
.upload-zone{border:2px dashed var(--border);border-radius:10px;padding:20px;text-align:center;cursor:pointer;transition:all .2s;margin-bottom:12px}
.upload-zone:hover{border-color:var(--accent);background:rgba(99,102,241,0.05)}
.result-item{padding:12px;border-left:3px solid var(--accent);margin-bottom:10px;background:var(--bg);border-radius:0 6px 6px 0}
.result-item .meta{font-size:11px;color:var(--muted);margin-bottom:4px}
.result-item .text{font-size:13px;line-height:1.6}
.hl{background:rgba(99,102,241,0.3);padding:1px 2px;border-radius:2px}
.answer-box{padding:16px;background:var(--bg);border-radius:8px;margin-bottom:12px;font-size:14px;line-height:1.7;white-space:pre-wrap}
.doc-item{display:flex;justify-content:space-between;align-items:center;padding:8px 10px;border:1px solid var(--border);border-radius:6px;margin-bottom:6px;font-size:12px}
.spinner{display:inline-block;width:16px;height:16px;border:2px solid var(--border);border-top-color:var(--accent);border-radius:50%;animation:spin .6s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}
#toast{position:fixed;top:20px;right:20px;z-index:9999}
.toast-msg{padding:10px 18px;border-radius:8px;font-size:12px;margin-bottom:8px}
.toast-success{background:#065f46;color:#6ee7b7}.toast-error{background:#7f1d1d;color:#fca5a5}
</style>
</head>
<body>
<div class="header">
  <h1>📚 RAG 文档问答引擎</h1>
  <div id="statusBar" class="status"><span class="spinner"></span> 检测服务中...</div>
</div>
<div id="toast"></div>
<div class="container">
  <div>
    <div class="card">
      <h3>📤 上传文档</h3>
      <div class="upload-zone" id="dropZone" onclick="document.getElementById('fileInput').click()">
        <p style="font-size:24px;margin-bottom:8px">📄</p>
        <p>点击或拖拽上传 PDF / DOCX / TXT</p>
        <p style="font-size:11px;margin-top:4px">自动解析 → 分块 → 索引</p>
      </div>
      <input type="file" id="fileInput" accept=".pdf,.docx,.doc,.txt" style="display:none" onchange="uploadFile()">
      <div id="uploadStatus" style="font-size:12px;color:var(--muted);margin-top:8px"></div>
    </div>
    <div class="card">
      <h3>📂 已上传文档</h3>
      <div id="docList" style="max-height:300px;overflow-y:auto"></div>
    </div>
  </div>
  <div>
    <div class="card">
      <h3>🔍 语义检索与问答</h3>
      <textarea id="queryInput" placeholder="输入关键词或问题，如：这份合同的有效期到什么时候？" onkeydown="if(event.key==='Enter'&&!event.shiftKey){event.preventDefault();search()}"></textarea>
      <div style="display:flex;gap:8px;margin-top:8px">
        <button class="btn btn-primary" style="width:auto;flex:1" onclick="search()">🔍 检索</button>
        <button class="btn btn-primary" style="width:auto;flex:1;background:var(--green)" onclick="qaStream()" id="qaBtn">🤖 AI 问答</button>
      </div>
    </div>
    <div id="resultsArea"><p style="text-align:center;color:var(--muted);margin:40px 0">输入问题开始搜索</p></div>
  </div>
</div>
<script>
var bp=window.location.pathname.replace(/\/+$/,'');
var API=(bp===''||bp==='/')?'/api':bp+'/api';
var status={tika:false,ollama:false,qa:false};

fetch(API+'/health').then(function(r){return r.json()}).then(function(d){
  status=d;
  var st=document.getElementById('statusBar');
  st.innerHTML='<span class="dot '+(d.tika?'dot-on':'dot-off')+'"></span> Tika <span class="dot '+(d.ollama?'dot-on':'dot-off')+'"></span> Ollama <span class="dot '+(d.qa?'dot-on':'dot-off')+'"></span> LLM';
  if(!d.qa) document.getElementById('qaBtn').textContent='🤖 问答(无LLM)';
  loadDocs();
}).catch(function(){document.getElementById('statusBar').innerHTML='<span class="dot dot-off"></span> 服务离线'});

var dz=document.getElementById('dropZone');
dz.ondragover=function(e){e.preventDefault();dz.style.borderColor='var(--accent)'};
dz.ondragleave=function(){dz.style.borderColor='var(--border)'};
dz.ondrop=function(e){e.preventDefault();dz.style.borderColor='var(--border)';var f=e.dataTransfer.files[0];if(f)doUpload(f)};
function uploadFile(){var f=document.getElementById('fileInput').files[0];if(f)doUpload(f)}

async function doUpload(file){
  document.getElementById('uploadStatus').innerHTML='<span class="spinner"></span> 解析中...';
  var fd=new FormData();fd.append('file',file);
  try{
    var r=await fetch(API+'/documents/upload',{method:'POST',body:fd});var d=await r.json();
    if(d.code===0){toast('上传成功: '+d.data.chunk_count+' 个文本块','success');loadDocs()}
    else{toast(d.message,'error')}
  }catch(e){toast(e.message,'error')}
  document.getElementById('uploadStatus').innerHTML='';
}

async function loadDocs(){
  try{
    var r=await fetch(API+'/documents');var docs=await r.json();
    var list=document.getElementById('docList');
    if(!docs.data||docs.data.length===0){list.innerHTML='<p style="font-size:12px;color:var(--muted)">暂无文档</p>';return}
    list.innerHTML=docs.data.map(function(d){
      return '<div class="doc-item"><span>📄 '+d.filename+'<br><span style="color:var(--muted)">'+d.chunk_count+' 个分块</span></span><button class="btn-danger" onclick="deleteDoc(\''+d.id+'\')">删除</button></div>';
    }).join('');
  }catch(e){}
}

async function deleteDoc(docID){
  if(!confirm('确认删除该文档？'))return;
  var r=await fetch(API+'/documents/'+docID,{method:'DELETE'});
  var d=await r.json();
  if(d.code===0){toast('已删除','success');loadDocs()}else{toast(d.message,'error')}
}

function highlight(text,query){
  var words=query.split(/\s+/).filter(function(w){return w.length>1});
  var t=text.substring(0,400);
  words.forEach(function(w){t=t.replace(new RegExp('('+w.replace(/[.*+?^$()|[\]\\]/g,'\\$&')+')','gi'),'<span class="hl">$1</span>')});
  return t;
}

async function search(){
  var q=document.getElementById('queryInput').value.trim();
  if(!q)return;
  var area=document.getElementById('resultsArea');
  area.innerHTML='<p style="text-align:center"><span class="spinner"></span> 语义检索中...</p>';
  try{
    var r=await fetch(API+'/search',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({query:q,top_k:5})});
    var d=await r.json();
    if(!d.data||!d.data.results||d.data.results.length===0){
      area.innerHTML='<p style="text-align:center;color:var(--muted);margin:40px">未找到相关内容<br><span style="font-size:12px">试试换个关键词或上传更多文档</span></p>';return
    }
    area.innerHTML='<h4 style="margin-bottom:12px">找到 '+d.data.total+' 条结果 ('+d.data.took_ms+'ms)</h4>'+
      d.data.results.map(function(r){return'<div class="result-item"><div class="meta">📄 '+r.doc_name+' | 相关度 '+(r.score*100).toFixed(0)+'%</div><div class="text">'+highlight(r.text,q)+'</div></div>'}).join('');
  }catch(e){area.innerHTML='<p style="color:var(--red)">搜索失败: '+e.message+'</p>'}
}

async function qaStream(){
  var q=document.getElementById('queryInput').value.trim();
  if(!q)return;
  var area=document.getElementById('resultsArea');
  area.innerHTML='<div class="answer-box" id="streamBox"><span class="spinner"></span> 正在思考...</div><div id="sourceBox"></div>';

  if(!status.qa){
    try{
      var r=await fetch(API+'/qa',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({question:q,top_k:5})});
      var d=await r.json();
      document.getElementById('streamBox').textContent=d.data.answer;
    }catch(e){document.getElementById('streamBox').textContent='错误: '+e.message}
    return;
  }

  try{
    var r2=await fetch(API+'/qa/stream',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({question:q,top_k:5})});
    var reader=r2.body.getReader();var decoder=new TextDecoder();
    var box=document.getElementById('streamBox');box.innerHTML='';
    while(true){
      var result=await reader.read();if(result.done)break;
      var lines=decoder.decode(result.value).split('\n');
      for(var i=0;i<lines.length;i++){
        if(lines[i].startsWith('data: ')){
          var data=lines[i].substring(6);
          if(data==='[DONE]'){box.innerHTML+='<br><span style="color:var(--muted);font-size:11px">回答完成</span>';return}
          try{box.innerHTML+=data.replace(/\\n/g,'<br>')}catch(e){box.innerHTML+=data}
        }
      }
    }
  }catch(e){document.getElementById('streamBox').innerHTML='错误: '+e.message}
}

function toast(msg,type){
  var e=document.getElementById('toast'),d=document.createElement('div');
  d.className='toast-msg toast-'+type;d.textContent=msg;e.appendChild(d);
  setTimeout(function(){d.remove()},3000);
}
</script></body></html>`
