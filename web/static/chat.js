const keyStore = {
  privateKey: null,
  publicKeys: {},
  sharedKeys: {}
};

const messageBuffer = [];
let isConnecting = false;

async function initKeyExchange(chatId, userId) {
  try {
    const keyExchangeClient = new KeyExchangeClient();
    
    const keyPair = await keyExchangeClient.generateKeyPair(chatId);
    keyStore.privateKey = keyPair.privateKey;
    
    await keyExchangeClient.sendPublicKey(chatId, userId, keyPair.publicKey);
    
    const peerKeys = await keyExchangeClient.getPeerKeys(chatId);
    
    for (const peer of peerKeys) {
      if (peer.clientId !== userId) {
        keyStore.publicKeys[peer.clientId] = peer.publicKey;
        keyStore.sharedKeys[peer.clientId] = computeSharedKey(
          keyStore.privateKey, 
          peer.publicKey
        );
      }
    }
    
    console.log("Key exchange completed successfully");
    return true;
  } catch (error) {
    console.error("Key exchange failed:", error);
    return false;
  }
}

document.getElementById("send-message").addEventListener("click", function() {
let messageInput = document.getElementById("message-input");
let messageText = messageInput.value.trim();
if (messageText === "") return;
const userId = localStorage.getItem('user_id');

sendMessageViaWebSocket(messageText); 
messageInput.value = "";
});


document.addEventListener('DOMContentLoaded', function () {
const chatId = localStorage.getItem('current_chat_id');
const currentUserId = localStorage.getItem('user_id');

fetch(`/get-chat?id=${chatId}`, {
    method: 'GET',
    headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`,
    },
})
.then(response => response.json())
.then(data => {
    if (data.success) {
        document.getElementById('chat-name').textContent = data.chat.name;
        
        const friends = data.chat.participants.filter(
            participant => participant.user_id !== currentUserId
        )
        .map(p => p.username);
        
        document.getElementById('friend-name').textContent = friends
            .join(', ');
    }
});
});


document.addEventListener('DOMContentLoaded', function () {
loadUserChats();
connectWebSocket();
loadHistory();
});

function loadUserChats() {
const userID = localStorage.getItem('user_id');

fetch('/list-user-chats', {
    method: 'GET',
    headers: {
        'Authorization': `Bearer ${localStorage.getItem('token')}`,
        'X-User-ID': userID,
    },
})
.then(response => {
    if (!response.ok) {
        return response.text().then(text => {
            throw new Error(text);
        });
    }
    return response.json();
})
.then(data => {
    if (data.success) {
        const chatList = document.getElementById('chat-list');
        chatList.innerHTML = '';
        const currentChatId = localStorage.getItem('current_chat_id');

        data.chats.forEach(chat => {
            const li = document.createElement('li');
            li.textContent = chat.name;
            li.dataset.chatId = chat.chat_id;

            if (chat.chat_id === currentChatId) {
                li.classList.add('active-chat');
            }
            const deleteIcon = document.createElement('span');
            deleteIcon.innerHTML = ' ⨉';
            deleteIcon.style.cursor = 'pointer';
            deleteIcon.style.float = 'right';
            deleteIcon.style.marginLeft = '10px';
            deleteIcon.addEventListener('click', (e) => {
                e.stopPropagation();
                deleteChat(chat.chat_id);
            });

            li.appendChild(deleteIcon);

            li.addEventListener('click', () => {
                localStorage.setItem('current_chat_id', chat.chat_id); 
                window.location.href = '/Kygram/chat';
            });
            chatList.appendChild(li);
        });
    } else {
        console.warn('No chats found for user.');
       // alert('Failed to load chats: ' + data.message);
    }
})
.catch(error => {
    console.error('Error loading chats:', error);
   // alert('Failed to load chats: ' + error.message);
});
}

document.addEventListener('DOMContentLoaded', function () {
const chatId = localStorage.getItem('current_chat_id');
if (!chatId) {
    alert('Chat not selected!');
    window.location.href = '/Kygram/dashboard';
    return;
}

fetch(`/get-chat?id=${chatId}`)
.then(response => response.json())
.then(data => {
if (data.success) {
    const chat = data.chat;
    document.getElementById('chat-name').textContent = chat.name;
    
    const currentUserId = localStorage.getItem('user_id');
    const friends = chat.participants
        .filter(p => p.user_id !== currentUserId)
        .map(p => p.username);
    
    document.getElementById('friend-name').textContent = friends.join(', ');
}
});

});

let ws = null;

function connectWebSocket() {
  const currentChatId = localStorage.getItem('current_chat_id');
  const UserId = localStorage.getItem('user_id');
  
  if (isConnecting) return;
  
  isConnecting = true;
  updateConnectionStatus('connecting');

  initKeyExchange(currentChatId, UserId).then(success => {

    ws = new WebSocket(`ws://localhost:2033/ws?chat_id=${currentChatId}&user_id=${UserId}`);
    
    ws.onopen = () => {
      console.log('WebSocket connected');
      updateConnectionStatus('connected');
      isConnecting = false;

      if (messageBuffer.length > 0) {
        console.log(`Sending ${messageBuffer.length} buffered messages`);
        messageBuffer.forEach(msg => {
          if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify(msg));
          }
        });
        messageBuffer.length = 0;
      }
    };

    ws.onmessage = (event) => {
      try {
          const data = JSON.parse(event.data);
          console.log("Received data:", data);

          if (data.message_type === "file") {
              let fileBytes;
              
              if (data.is_base64) {
                  const binary = atob(data.message);
                  fileBytes = new Uint8Array(binary.length);
                  for (let i = 0; i < binary.length; i++) {
                      fileBytes[i] = binary.charCodeAt(i);
                  }
                  //console.log("Decoded Base64 data, size:", fileBytes.length);
              } else {
                  fileBytes = Array.isArray(data.message) ? 
                      new Uint8Array(data.message) : 
                      new Uint8Array(Object.values(data.message));
              }
              
              const fileExtension = data.file_name.split('.').pop().toLowerCase();
              const mimeTypes = {
                  'png': 'image/png',
                  'jpg': 'image/jpeg',
                  'jpeg': 'image/jpeg',
                  'gif': 'image/gif',
                  'pdf': 'application/pdf',
                  'txt': 'text/plain',
                  'doc': 'application/msword',
                  'docx': 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
                  'xls': 'application/vnd.ms-excel',
                  'xlsx': 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
                  'zip': 'application/zip',
                  'rar': 'application/x-rar-compressed'
              };
              const mimeType = mimeTypes[fileExtension] || 'application/octet-stream';

              const blob = new Blob([fileBytes], { type: mimeType });
              const fileUrl = URL.createObjectURL(blob);
              
              console.log(`Created blob with size: ${blob.size} bytes and type: ${mimeType}`);
              
              appendMessage(
                  data.sender_name,
                  {
                      type: 'file',
                      url: fileUrl,
                      fileName: data.file_name,
                      mimeType: mimeType
                  },
                  data.created_at,
                  data.sender_id
              );
          } else {
              appendMessage(data.sender_name, data.message, data.created_at, data.sender_id);
          }
      } catch (e) {
          console.error('Error processing message:', e);
          console.error('Raw message:', event.data);
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
      updateConnectionStatus('disconnected');
      isConnecting = false;
    };

    ws.onclose = (event) => {
      console.error('WebSocket closed:', event.code, event.reason);
      console.log('WebSocket disconnected');
      updateConnectionStatus('disconnected');
      isConnecting = false;

      setTimeout(() => {
        console.log('Reconnecting WebSocket...');
        connectWebSocket();
      }, 3000);
    };
  }).catch(error => {
    console.error('Key exchange error:', error);
    isConnecting = false;
    updateConnectionStatus('disconnected');
    
    setTimeout(() => {
      console.log('Reconnecting without key exchange...');
      connectWebSocket();
    }, 3000);
  });
}

function sendMessageViaWebSocket(messageText) {
  const message = {
    type: "text",
    text: messageText,
  };
  
  console.log("Preparing message:", message);
  
  if (!ws || !isWebSocketReady()) {
    console.log('WebSocket not ready, buffering message');
    messageBuffer.push(message);
    
    if (!isConnecting) {
      console.log('Initiating WebSocket connection...');
      connectWebSocket();
    }
    return;
  }
  
  console.log("Sending message via WebSocket:", message);
  ws.send(JSON.stringify(message));
}

function isWebSocketReady() {
  return ws && ws.readyState === WebSocket.OPEN;
}

function updateConnectionStatus(status) {
  const statusElement = document.getElementById('connection-status');
  if (statusElement) {
    if (status === 'connecting') {
      statusElement.textContent = 'Connecting...';
      statusElement.setAttribute('data-status', 'connecting');
    } else {
      statusElement.textContent = status === 'connected' ? 'Connected' : 'Disconnected';
      statusElement.setAttribute('data-status', status);
    }
  }
  
  if (!statusElement) {
    const status = document.createElement('div');
    status.id = 'connection-status';
    status.style.position = 'fixed';
    status.style.top = '10px';
    status.style.left = '10px';
    status.style.padding = '5px 10px';
    status.style.borderRadius = '5px';
    status.style.fontSize = '0.9em';
    document.body.appendChild(status);
    
    updateConnectionStatus(status);
  }
}

function sendFile(file) {
  const chunkSize = 64 * 1024;
  let offset = 0;
  const reader = new FileReader();
  const totalChunks = Math.ceil(file.size / chunkSize);

  console.log(`File size: ${file.size}, Chunk size: ${chunkSize}, Total chunks: ${totalChunks}`);

  reader.onload = function(e) {
    const chunk = e.target.result;
    if (!(chunk instanceof ArrayBuffer)) {
      console.error("Invalid chunk type:", typeof chunk);
      return;
    }

    console.log(`Chunk length: ${chunk.byteLength}, Offset: ${offset}`);
    
    const byteArray = new Uint8Array(chunk);
    
    const chunkArray = Array.from(byteArray);

    const message = {
      type: "file",
      file_name: file.name,
      chunk_index: Math.floor(offset / chunkSize),
      total_chunks: totalChunks,
      data: chunkArray,
      mime_type: file.type
    };

    console.log(`Sending file chunk ${message.chunk_index + 1} of ${totalChunks}`);
    
    if (!isWebSocketReady()) {
      console.log('WebSocket not ready, buffering file chunk');
      messageBuffer.push(message);
      
      if (!isConnecting) {
        console.log('Initiating WebSocket connection for file transfer...');
        connectWebSocket();
      }
      
      return;
    }
    
    ws.send(JSON.stringify(message));

    offset += chunk.byteLength;
    console.log(`New offset: ${offset}, Progress: ${Math.floor((offset / file.size) * 100)}%`);
    
    if (offset < file.size) {
      readNextChunk();
    } else {
      console.log("File upload complete");
    }
  };

  function readNextChunk() {
    const slice = file.slice(offset, offset + chunkSize);
    reader.readAsArrayBuffer(slice);
  }

  if (isWebSocketReady()) {
    readNextChunk();
  } else {
    console.log('WebSocket not ready, will buffer the first chunk once read');
    const firstSlice = file.slice(0, chunkSize);
    reader.readAsArrayBuffer(firstSlice);
    
    if (!isConnecting) {
      connectWebSocket();
    }
  }
}

function appendMessage(senderName, messageText, timestamp, senderId) {
    const messagesDiv = document.getElementById('chat-messages');
    const msgElement = document.createElement('div');
    const userId = localStorage.getItem('user_id'); 
    msgElement.className = 'message';

    const dateObj = new Date(timestamp);
    dateObj.setHours(dateObj.getHours());
    const messageDate = dateObj.toLocaleDateString();
    const timeString = dateObj.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

    const lastDateHeader = [...messagesDiv.children]
        .reverse()
        .find(el => el.classList.contains('date-header'));

    if (!lastDateHeader || lastDateHeader.textContent !== messageDate) {
        const dateHeader = document.createElement('div');
        dateHeader.className = 'date-header';
        dateHeader.textContent = messageDate;
        messagesDiv.appendChild(dateHeader);
    }

    const isCurrentUser = senderId === userId;
    msgElement.className = isCurrentUser ? 'message my-message' : 'message other-message';

    if (typeof messageText === 'object' && messageText.type === 'file') {
        const { url, fileName, mimeType } = messageText;
        const isImage = mimeType.startsWith('image/');
        
        let filePreview = '';
        if (isImage) {
            filePreview = `<img src="${url}" alt="${fileName}" style="max-width: 200px; max-height: 200px; margin: 5px 0;">`;
        }

        msgElement.innerHTML = `
            <span class="sender">${senderName}</span>
            <span class="text">
                ${filePreview}
                <a href="${url}" 
                   download="${fileName}"
                   class="file-download">
                    📎 ${fileName}
                </a>
            </span>
            <span class="time">${timeString}</span>
        `;

        const link = msgElement.querySelector('a');
        link.addEventListener('click', () => {
            setTimeout(() => {
                URL.revokeObjectURL(url);
            }, 1000);
        });
    } else {
        msgElement.innerHTML = `
            <span class="sender">${senderName}</span>
            <span class="text">${messageText}</span>
            <span class="time">${timeString}</span>
        `;
    }

    messagesDiv.appendChild(msgElement);
    setTimeout(() => {
        msgElement.classList.add('visible');
    }, 10);
    messagesDiv.scrollTop = messagesDiv.scrollHeight;
}

document.addEventListener('DOMContentLoaded', () => {
  console.log("DOM loaded, initializing WebSocket connection...");
  connectWebSocket();
  loadHistory();

  const statusElement = document.getElementById('connection-status');
  if (!statusElement) {
    const status = document.createElement('div');
    status.id = 'connection-status';
    status.style.position = 'fixed';
    status.style.top = '10px';
    status.style.left = '10px';
    status.style.padding = '5px 10px';
    status.style.borderRadius = '5px';
    status.style.fontSize = '0.9em';
    document.body.appendChild(status);
    updateConnectionStatus('connecting');
  }
});

document.getElementById("file-input").addEventListener("change", function(event) {
    const file = event.target.files[0];
    if (file) {
        console.log("File upload successful");
        sendFile(file);
    }
});

async function loadHistory() {
try {
    const response = await fetch(`/messages?chat_id=${localStorage.getItem('current_chat_id')}`);
    if (!response.ok) {
        throw new Error(`HTTP error! Status: ${response.status}`);
    }
    const data = await response.json();
    //console.log("Loaded messages:", data);

    const chatMessages = document.getElementById('chat-messages');
    chatMessages.innerHTML = '';

    if (data.messages && Array.isArray(data.messages)) {
        if (data.messages.length === 0) {
            console.log("No messages found in this chat.");
            return;
        }
        data.messages.forEach(msg => {
           //console.log("Message data:", msg);
           const localTime = new Date(msg.created_at);
           localTime.setHours(localTime.getHours() - 3);
           
        if (msg.message_type === 'file') {
            let fileBytes;
            
            if (msg.is_base64) {
                const binary = atob(msg.message);
                fileBytes = new Uint8Array(binary.length);
                for (let i = 0; i < binary.length; i++) {
                    fileBytes[i] = binary.charCodeAt(i);
                }
                //console.log("Decoded Base64 history data, size:", fileBytes.length);
            } else {
                fileBytes = Array.isArray(msg.message) ? 
                    new Uint8Array(msg.message) : 
                    new Uint8Array(Object.values(msg.message));
            }
            
            const fileExtension = msg.file_name.split('.').pop().toLowerCase();
            const mimeTypes = {
                'png': 'image/png',
                'jpg': 'image/jpeg',
                'jpeg': 'image/jpeg',
                'gif': 'image/gif',
                'pdf': 'application/pdf',
                'txt': 'text/plain',
                'doc': 'application/msword',
                'docx': 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
                'xls': 'application/vnd.ms-excel',
                'xlsx': 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
                'zip': 'application/zip',
                'rar': 'application/x-rar-compressed'
            };
            const mimeType = mimeTypes[fileExtension] || 'application/octet-stream';

            const blob = new Blob([fileBytes], { type: mimeType });
            const fileUrl = URL.createObjectURL(blob);
            
            //console.log(`Created history blob with size: ${blob.size} bytes and type: ${mimeType}`);

            appendMessage(
                msg.sender_name,
                {
                    type: 'file',
                    url: fileUrl,
                    fileName: msg.file_name,
                    mimeType: mimeType
                },
                localTime,
                msg.sender_id
            );
        } else {
            appendMessage(
                msg.sender_name,
                msg.text,
                localTime,
                msg.sender_id
            );
        }
    });
    } else {
        console.error('Invalid response format:', data);
        //alert('Failed to load chat history: Invalid response format');
    }
} catch (error) {
    console.error('Error loading chat history:', error);
    alert('Failed to load chat history: ' + error.message);
}
}


document.getElementById("message-input").addEventListener("keydown", function (event) {
if (event.key === "Enter") {
    event.preventDefault(); 
    document.getElementById("send-message").click();
}
});

async function deleteChat(chatId) {
if (!confirm('Are you sure you want to delete this chat?')) {
    return;
}

try {
    const response = await fetch('/close-chat', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
        body: JSON.stringify({
            chat_id: chatId,
        }),
    });

    const data = await response.json();
    if (data.success) {
        //alert('Chat deleted successfully!');
        loadUserChats();
    } else {
        alert('Error deleting chat: ' + (data.message || 'Unknown error'));
    }
} catch (error) {
    console.error('Delete error:', error);
    alert('Failed to delete chat');
}
}
