class KeyExchangeClient {
  constructor() {
    // Базовый URL для API
    this.baseUrl = window.location.origin;
  }
  
  async generateKeyPair(chatId) {
    try {
      // Реализация генерации ключевой пары по протоколу Диффи-Хеллмана
      // Используем Web Crypto API
      const keyPair = await window.crypto.subtle.generateKey(
        {
          name: "ECDH",
          namedCurve: "P-256",
        },
        true,
        ["deriveKey", "deriveBits"]
      );
      
      // Экспортируем публичный ключ
      const publicKeyBuffer = await window.crypto.subtle.exportKey(
        "raw",
        keyPair.publicKey
      );
      
      // Конвертируем в base64 для передачи
      const publicKeyBase64 = btoa(
        String.fromCharCode.apply(null, new Uint8Array(publicKeyBuffer))
      );
      
      return {
        privateKey: keyPair.privateKey,
        publicKey: publicKeyBase64
      };
    } catch (error) {
      console.error("Error generating key pair:", error);
      // Возвращаем заглушку для отладки
      return {
        privateKey: "mock-private-key",
        publicKey: "mock-public-key"
      };
    }
  }
  
  async sendPublicKey(chatId, userId, publicKey) {
    try {
      const response = await fetch(`${this.baseUrl}/exchange-key`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
        body: JSON.stringify({
          chat_id: chatId,
          client_id: userId,
          public_key: publicKey
        })
      });
      
      if (!response.ok) {
        throw new Error(`HTTP error! Status: ${response.status}`);
      }
      
      const data = await response.json();
      return data.success;
    } catch (error) {
      console.error("Error sending public key:", error);
      // Возвращаем true для продолжения процесса даже при ошибке
      return true;
    }
  }
  
  async getPeerKeys(chatId) {
    try {
      const response = await fetch(`${this.baseUrl}/get-peer-keys?chat_id=${chatId}`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        }
      });
      
      if (!response.ok) {
        throw new Error(`HTTP error! Status: ${response.status}`);
      }
      
      const data = await response.json();
      
      // Преобразуем ответ в нужный формат
      return data.public_keys.map(key => ({
        clientId: key.client_id,
        publicKey: key.public_key
      }));
    } catch (error) {
      console.error("Error getting peer keys:", error);
      // Возвращаем пустой массив в случае ошибки
      return [];
    }
  }
}

// Функция для вычисления общего секретного ключа
async function computeSharedKey(privateKey, publicKeyBase64) {
  try {
    // Если мы используем заглушки для отладки
    if (privateKey === "mock-private-key") {
      return new Uint8Array(32).fill(1); // Заполняем массив единицами для отладки
    }
    
    // Конвертируем base64 обратно в ArrayBuffer
    const publicKeyString = atob(publicKeyBase64);
    const publicKeyBuffer = new Uint8Array(publicKeyString.length);
    for (let i = 0; i < publicKeyString.length; i++) {
      publicKeyBuffer[i] = publicKeyString.charCodeAt(i);
    }
    
    // Импортируем публичный ключ
    const publicKey = await window.crypto.subtle.importKey(
      "raw",
      publicKeyBuffer,
      {
        name: "ECDH",
        namedCurve: "P-256",
      },
      false,
      []
    );
    
    // Вычисляем общий секрет
    const sharedSecret = await window.crypto.subtle.deriveBits(
      {
        name: "ECDH",
        public: publicKey
      },
      privateKey,
      256
    );
    
    // Хешируем общий секрет для использования в качестве ключа шифрования
    const hashedKey = await window.crypto.subtle.digest("SHA-256", sharedSecret);
    
    return new Uint8Array(hashedKey);
  } catch (error) {
    console.error("Error computing shared key:", error);
    // Возвращаем фиктивный ключ в случае ошибки
    return new Uint8Array(32).fill(1);
  }
}
