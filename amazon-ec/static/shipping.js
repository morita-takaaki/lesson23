"use strict";
const byId = id => document.getElementById(id);
async function api(path, options={}) {
 const res=await fetch(path,{credentials:"same-origin",...options});
 if(!res.ok) throw new Error((await res.text()).trim()||`HTTP ${res.status}`);
 return res.json();
}
async function load(){
 const orders=await api("/api/admin/shipments");const tbody=byId("shipments");tbody.replaceChildren();
 for(const order of orders){
  const tr=document.createElement("tr");
  for(const value of [order.id,order.product_name,order.quantity,order.created_at,order.payment_status==="paid"?"決済済み":"未決済",order.shipping_status]){
   const td=document.createElement("td");td.textContent=String(value);tr.appendChild(td);
  }
  const td=document.createElement("td");
  if(order.payment_status==="paid"&&order.shipping_status==="受付"){
   const btn=document.createElement("button");btn.type="button";btn.textContent="発送済みにする";
   btn.addEventListener("click",async()=>{
    if(!confirm(`注文 ${order.id} を発送済みにしますか？`))return;
    btn.disabled=true;
    try{await api("/api/admin/shipments/dispatch",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({order_id:order.id})});await load();byId("message").textContent="発送状態を更新しました。";}
    catch(e){byId("message").textContent=e.message;await load().catch(()=>{});}
   });td.appendChild(btn);
  }else{td.textContent="－";}
  tr.appendChild(td);tbody.appendChild(tr);
 }
}
async function init(){try{const me=await api("/api/me");if(!me.is_admin)throw new Error("管理者権限がありません");byId("panel").hidden=false;byId("message").textContent=`管理者：${me.username}`;await load();}catch(e){byId("message").textContent=e.message;}}
byId("reload").addEventListener("click",()=>load().catch(e=>byId("message").textContent=e.message));
init();
