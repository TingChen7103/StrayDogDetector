import { createApp } from "vue";
import App from "@/App.vue";
import * as store from '@/store';

// vuetify
import 'vuetify/styles';
import { createVuetify } from 'vuetify';
import { aliases, mdi } from 'vuetify/iconsets/mdi-svg';


const vuetify = createVuetify({
    theme: {
        defaultTheme: 'dark',
    },
    icons: {
        defaultSet: 'mdi',
        aliases,
        sets: {
            mdi,
        },
    },
});

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
    .use(vuetify)
    .mount("#app");
