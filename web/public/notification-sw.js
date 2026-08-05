self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const targetPath = event.notification.data?.path || "/notifications";
  const targetURL = new URL(targetPath, self.location.origin).href;
  event.waitUntil(self.clients.matchAll({ type: "window", includeUncontrolled: true }).then(async (clientList) => {
    const client = clientList.find((item) => new URL(item.url).origin === self.location.origin);
    if (client) {
      await client.navigate(targetURL);
      return client.focus();
    }
    return self.clients.openWindow(targetURL);
  }));
});
