import { createApp } from "vue";
import App from "@/App.vue";
import * as store from '@/store';

import 'bootstrap/dist/css/bootstrap.min.css';

const app = createApp(App);

// register global variables
for (const _key in store) {
    const key = _key as keyof typeof store;
    app.provide(key, store[key]);
}

app.config.errorHandler = (err) => {
    alert(err);
    console.error(err);
}

app
    .mount("#app");
