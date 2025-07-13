import { Bot } from 'grammy'
 import axios from 'axios'
 import dotenv from 'dotenv'
 
 dotenv.config()
 
 const bot = new Bot(process.env.BOT_TOKEN)
 
 const allowedChatIds = (process.env.ALLOWED_CHAT_IDS || '')
   .split(',')
   .map((id) => parseInt(id.trim(), 10))
 
 let tokenCache = { token: '', expires: 0 }
 
 async function getQLToken() {
   const now = Date.now()
   if (tokenCache.token && now < tokenCache.expires - 60000) return tokenCache.token
 
   const { data } = await axios.get(
     `${process.env.QL_BASE_URL}/open/auth/token?client_id=${process.env.QL_CLIENT_ID}&client_secret=${process.env.QL_CLIENT_SECRET}`
   )
 
   tokenCache.token = data.data.token
   tokenCache.expires = now + data.data.expiration * 1000
   return tokenCache.token
 }
 
 async function addOrUpdateEnv(name, value) {
   const token = await getQLToken()
   const headers = { Authorization: `Bearer ${token}` }
 
   // 查找是否已存在
   const { data: searchRes } = await axios.get(`${process.env.QL_BASE_URL}/open/envs?searchValue=${name}`, { headers })
   const exists = searchRes.data.find((item) => item.name === name)
 
   if (exists) {
     // 更新
     await axios.put(`${process.env.QL_BASE_URL}/open/envs`, {
       name,
       value,
       _id: exists._id,
     }, { headers })
     return 'updated'
   } else {
     // 添加
     await axios.post(`${process.env.QL_BASE_URL}/open/envs`, [{ name, value }], { headers })
     return 'created'
   }
 }
 
 bot.on('message:text', async (ctx) => {
   const { chat, text } = ctx.message
   const chatId = chat.id
 
   if (!allowedChatIds.includes(chatId)) return
 
   const matches = [...text.matchAll(/export\s+(\w+)=["']([^"']+)["']/g)]
   for (const match of matches) {
     const [_, name, value] = match
     try {
       const result = await addOrUpdateEnv(name, value)
       await ctx.reply(`✅ 变量 ${name} ${result} 成功`)
     } catch (e) {
       await ctx.reply(`❌ 添加变量 ${name} 失败：${e.message}`)
       console.error(e)
     }
   }
 })
 
 bot.start()
 
