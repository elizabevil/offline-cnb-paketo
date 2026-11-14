const http = require('http');
const port = process.env.PORT || 8080;
http.createServer((_, res) => { res.end('ok from node fixture'); }).listen(port, () => {
  console.log('listening on ' + port);
});
