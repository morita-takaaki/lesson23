"use strict";
let products = [];
const $ = id => document.getElementById(id);
async function api(path, options={}) {
 const res=await fetch(path,{credentials:"same-origin",...options});
 if(!res.ok) throw new Error((await res.text()).trim() || `HTTP ${res.status}`);
 return res.json();
}
async function init() {
 try {
  const me=await api("/api/me");
  if(!me.is_admin) {$("admin-status").textContent="管理者権限がありません。";return;}
  $("admin-status").textContent=`管理者：${me.username}`;
  $("admin-panel").hidden=false;
  await load();
 }catch(e){$("admin-status").textContent="管理者としてログインしてください。";}
}
async function load(){
 products=await api("/api/admin/products");
 const body=$("products-body");body.replaceChildren();
 for(const p of products){
  const tr=document.createElement("tr");
  const values=[p.id,"",p.name,`¥${p.price.toLocaleString()}`,p.stock,p.is_active?"公開":"非表示",""];
  for(const v of values){const td=document.createElement("td");td.textContent=String(v);tr.appendChild(td);}
  if(p.image_url){const img=document.createElement("img");img.src=p.image_url;img.alt="商品画像";tr.children[1].appendChild(img);}
  if(p.is_active){
   const edit=document.createElement("button");edit.textContent="変更";edit.onclick=()=>editProduct(p);tr.lastChild.appendChild(edit);
   const hide=document.createElement("button");hide.textContent="非表示";hide.onclick=()=>hideProduct(p);tr.lastChild.appendChild(hide);
  }
  body.appendChild(tr);
 }
}
function editProduct(p){
 $("edit-id").value=p.id;$("name").value=p.name;$("price").value=p.price;$("stock").value=p.stock;
 $("image").value="";$("image-current").textContent=p.image_filename?`登録済み画像：${p.image_filename}`:"画像なし";
 $("form-title").textContent=`商品変更（ID ${p.id}）`;window.scrollTo(0,0);
}
function resetForm(){ $("product-form").reset();$("edit-id").value="";$("image-current").textContent="";$("form-title").textContent="商品登録"; }
async function hideProduct(p){
 if(!confirm(`「${p.name}」を非表示にしますか？`))return;
 try{await api(`/api/admin/product?id=${p.id}`,{method:"DELETE"});await load();}catch(e){alert(e.message);}
}
async function save(e){
 e.preventDefault();const id=$("edit-id").value;const old=products.find(p=>String(p.id)===id);
 let filename=old?.image_filename||"";
 try{
  const file=$("image").files[0];
  if(file){const fd=new FormData();fd.append("image",file);const upload=await api("/api/admin/upload",{method:"POST",body:fd});filename=upload.image_filename;}
  const payload={name:$("name").value.trim(),price:Number($("price").value),stock:Number($("stock").value),image_filename:filename};
  await api(id?`/api/admin/product?id=${id}`:"/api/admin/products",{method:id?"PUT":"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
  resetForm();await load();alert("保存しました");
 }catch(err){alert("保存できませんでした："+err.message);}
}
$("product-form").addEventListener("submit",save);$("reset-btn").addEventListener("click",resetForm);init();
