(window.webpackJsonp=window.webpackJsonp||[]).push([[24],{128:function(e,t,n){"use strict";n.d(t,"a",(function(){return d})),n.d(t,"b",(function(){return f}));var r=n(0),i=n.n(r);function o(e,t,n){return t in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function a(e,t){var n=Object.keys(e);if(Object.getOwnPropertySymbols){var r=Object.getOwnPropertySymbols(e);t&&(r=r.filter((function(t){return Object.getOwnPropertyDescriptor(e,t).enumerable}))),n.push.apply(n,r)}return n}function c(e){for(var t=1;t<arguments.length;t++){var n=null!=arguments[t]?arguments[t]:{};t%2?a(Object(n),!0).forEach((function(t){o(e,t,n[t])})):Object.getOwnPropertyDescriptors?Object.defineProperties(e,Object.getOwnPropertyDescriptors(n)):a(Object(n)).forEach((function(t){Object.defineProperty(e,t,Object.getOwnPropertyDescriptor(n,t))}))}return e}function u(e,t){if(null==e)return{};var n,r,i=function(e,t){if(null==e)return{};var n,r,i={},o=Object.keys(e);for(r=0;r<o.length;r++)n=o[r],t.indexOf(n)>=0||(i[n]=e[n]);return i}(e,t);if(Object.getOwnPropertySymbols){var o=Object.getOwnPropertySymbols(e);for(r=0;r<o.length;r++)n=o[r],t.indexOf(n)>=0||Object.prototype.propertyIsEnumerable.call(e,n)&&(i[n]=e[n])}return i}var l=i.a.createContext({}),p=function(e){var t=i.a.useContext(l),n=t;return e&&(n="function"==typeof e?e(t):c(c({},t),e)),n},d=function(e){var t=p(e.components);return i.a.createElement(l.Provider,{value:t},e.children)},s={inlineCode:"code",wrapper:function(e){var t=e.children;return i.a.createElement(i.a.Fragment,{},t)}},b=i.a.forwardRef((function(e,t){var n=e.components,r=e.mdxType,o=e.originalType,a=e.parentName,l=u(e,["components","mdxType","originalType","parentName"]),d=p(n),b=r,f=d["".concat(a,".").concat(b)]||d[b]||s[b]||o;return n?i.a.createElement(f,c(c({ref:t},l),{},{components:n})):i.a.createElement(f,c({ref:t},l))}));function f(e,t){var n=arguments,r=t&&t.mdxType;if("string"==typeof e||r){var o=n.length,a=new Array(o);a[0]=b;var c={};for(var u in t)hasOwnProperty.call(t,u)&&(c[u]=t[u]);c.originalType=e,c.mdxType="string"==typeof e?e:r,a[1]=c;for(var l=2;l<o;l++)a[l]=n[l];return i.a.createElement.apply(null,a)}return i.a.createElement.apply(null,n)}b.displayName="MDXCreateElement"},96:function(e,t,n){"use strict";n.r(t),n.d(t,"frontMatter",(function(){return a})),n.d(t,"metadata",(function(){return c})),n.d(t,"toc",(function(){return u})),n.d(t,"default",(function(){return p}));var r=n(3),i=n(7),o=(n(0),n(128)),a={id:"config-auth",title:"Auth Configuration",sidebar_label:"Auth"},c={unversionedId:"config-auth",id:"config-auth",isDocsHomePage:!1,title:"Auth Configuration",description:"Auth is only configurable in the Enterprise version of BuildBuddy.",source:"@site/../docs/config-auth.md",slug:"/config-auth",permalink:"/docs/config-auth",editUrl:"https://github.com/buildbuddy-io/buildbuddy/edit/master/docs/../docs/config-auth.md",version:"current",sidebar_label:"Auth",sidebar:"someSidebar",previous:{title:"SSL Configuration",permalink:"/docs/config-ssl"},next:{title:"Integration Configuration",permalink:"/docs/config-integrations"}},u=[{value:"Section",id:"section",children:[]},{value:"Options",id:"options",children:[]},{value:"Redirect URL",id:"redirect-url",children:[]},{value:"Google auth provider",id:"google-auth-provider",children:[]},{value:"Example section",id:"example-section",children:[]}],l={toc:u};function p(e){var t=e.components,n=Object(i.a)(e,["components"]);return Object(o.b)("wrapper",Object(r.a)({},l,n,{components:t,mdxType:"MDXLayout"}),Object(o.b)("p",null,"Auth is only configurable in the ",Object(o.b)("a",{parentName:"p",href:"/docs/enterprise"},"Enterprise version")," of BuildBuddy."),Object(o.b)("h2",{id:"section"},"Section"),Object(o.b)("p",null,Object(o.b)("inlineCode",{parentName:"p"},"auth:")," The Auth section enables BuildBuddy authentication using an OpenID Connect provider that you specify. ",Object(o.b)("strong",{parentName:"p"},"Optional")),Object(o.b)("h2",{id:"options"},"Options"),Object(o.b)("ul",null,Object(o.b)("li",{parentName:"ul"},Object(o.b)("inlineCode",{parentName:"li"},"oauth_providers:")," A list of configured OAuth Providers.",Object(o.b)("ul",{parentName:"li"},Object(o.b)("li",{parentName:"ul"},Object(o.b)("inlineCode",{parentName:"li"},"issuer_url: ")," The issuer URL of this OIDC Provider."),Object(o.b)("li",{parentName:"ul"},Object(o.b)("inlineCode",{parentName:"li"},"client_id: ")," The oauth client ID."),Object(o.b)("li",{parentName:"ul"},Object(o.b)("inlineCode",{parentName:"li"},"client_secret: ")," The oauth client secret."))),Object(o.b)("li",{parentName:"ul"},Object(o.b)("inlineCode",{parentName:"li"},"enable_anonymous_usage:")," If true, unauthenticated build uploads will still be allowed but won't be associated with your organization.")),Object(o.b)("h2",{id:"redirect-url"},"Redirect URL"),Object(o.b)("p",null,"If during your OpenID provider configuration you're asked to enter a ",Object(o.b)("strong",{parentName:"p"},"Redirect URL"),", you should enter ",Object(o.b)("inlineCode",{parentName:"p"},"https://YOUR_BUILDBUDDY_URL/auth/"),". For example if your BuildBuddy instance was hosted on ",Object(o.b)("inlineCode",{parentName:"p"},"https://buildbuddy.acme.com"),", you'd enter ",Object(o.b)("inlineCode",{parentName:"p"},"https://buildbuddy.acme.com/auth/")," as your redirect url."),Object(o.b)("h2",{id:"google-auth-provider"},"Google auth provider"),Object(o.b)("p",null,"If you'd like to use Google as an auth provider, you can easily obtain your client id and client secret ",Object(o.b)("a",{parentName:"p",href:"https://console.developers.google.com/apis/credentials"},"here"),"."),Object(o.b)("h2",{id:"example-section"},"Example section"),Object(o.b)("pre",null,Object(o.b)("code",{parentName:"pre"},'auth:\n  oauth_providers:\n    - issuer_url: "https://accounts.google.com"\n      client_id: "12345678911-f1r0phjnhbabcdefm32etnia21keeg31.apps.googleusercontent.com"\n      client_secret: "sEcRetKeYgOeShErE"\n')))}p.isMDXComponent=!0}}]);