Мессенджер реализует симметричные алгоритмы шифрования RC5 и Twofish, а также протокол обмена ключами 
Диффи-Хеллмана. 

Для обеспечения безопасности передаваемых данных применены различные режимы 
блочного шифрования, включая ECB, CBC, PCBC, CFB, OFB, CTR и Random Delta, а также 
методы набивки Zeros, ANSI X.923, PKCS7, ISO 10126. 

Архитектура приложения построена на клиент-серверной модели, где серверная часть реализована с 
использованием gRPC, PostgreSQL, Redis и RabbitMQ, развернутых в Docker. Сервер отвечает за 
обработку запросов, управление сеансовыми ключами и маршрутизацию зашифрованных 
сообщений между клиентами. Клиентская часть представляет собой веб-приложение на HTML, 
CSS и JavaScript, использующее WebSocket для потоковой передачи данных.

/ The messenger implements symmetric encryption algorithms RC5 and Twofish, as well as the Diffie-Hellman key exchange protocol.

/ To ensure the security of transmitted data, various block encryption modes are used, 
including ECB, CBC, PCBC, CFB, OFB, CTR and Random Delta, as well as Zeros, ANSI X.923, PKCS7, ISO 10126 padding methods.

/The application architecture is built on a client-server model, where the server part is implemented using gRPC, PostgreSQL, Redis and RabbitMQ deployed in Docker. 
The server is responsible for processing requests, managing session keys and routing encrypted messages between clients. 
The client part is a web application in HTML, CSS and JavaScript, using WebSocket for streaming data.

Функционал web-приложения (Web application functionality):
![image](https://github.com/user-attachments/assets/67d8f9ff-3bb0-4870-ba4b-43f35ec928cc)
![image](https://github.com/user-attachments/assets/6b4a8037-c5cb-4fb8-a521-293a963acf07)
![image](https://github.com/user-attachments/assets/ddd03be3-afd0-4623-a6ef-f2d5798bfa4a)
