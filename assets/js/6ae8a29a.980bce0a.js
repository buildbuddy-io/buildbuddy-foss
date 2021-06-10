(window.webpackJsonp=window.webpackJsonp||[]).push([[28],{100:function(e,t,n){"use strict";n.r(t),n.d(t,"frontMatter",(function(){return r})),n.d(t,"metadata",(function(){return u})),n.d(t,"toc",(function(){return c})),n.d(t,"default",(function(){return d}));var i=n(3),a=n(7),o=(n(0),n(128)),r={id:"guide-auth",title:"Authentication Guide",sidebar_label:"Authentication Guide"},u={unversionedId:"guide-auth",id:"guide-auth",isDocsHomePage:!1,title:"Authentication Guide",description:"You have two choices for authenticating your BuildBuddy requests:",source:"@site/../docs/guide-auth.md",slug:"/guide-auth",permalink:"/docs/guide-auth",editUrl:"https://github.com/buildbuddy-io/buildbuddy/edit/master/docs/../docs/guide-auth.md",version:"current",sidebar_label:"Authentication Guide",sidebar:"someSidebar",previous:{title:"Guides",permalink:"/docs/guides"},next:{title:"Build Metadata Guide",permalink:"/docs/guide-metadata"}},c=[{value:"API key",id:"api-key",children:[{value:"Separate auth file",id:"separate-auth-file",children:[]},{value:"Command line",id:"command-line",children:[]}]},{value:"Certificate",id:"certificate",children:[]}],l={toc:c};function d(e){var t=e.components,n=Object(a.a)(e,["components"]);return Object(o.b)("wrapper",Object(i.a)({},l,n,{components:t,mdxType:"MDXLayout"}),Object(o.b)("p",null,"You have two choices for authenticating your BuildBuddy requests:"),Object(o.b)("ul",null,Object(o.b)("li",{parentName:"ul"},"API key"),Object(o.b)("li",{parentName:"ul"},"certificate based mTLS auth")),Object(o.b)("p",null,"Both of these choices require you to ",Object(o.b)("a",{parentName:"p",href:"https://app.buildbuddy.io/"},"create a BuildBuddy account"),"."),Object(o.b)("h2",{id:"api-key"},"API key"),Object(o.b)("p",null,"This the simpler of the two methods. It passes an API key along with all grpcs requests that is associated with your BuildBuddy organization. This key can be used by anyone in your organization, as it ties builds to your org - not your individual user."),Object(o.b)("p",null,"You can find API key authenticated URLs on your ",Object(o.b)("a",{parentName:"p",href:"https://app.buildbuddy.io/docs/setup/"},"setup instructions")," once you've ",Object(o.b)("a",{parentName:"p",href:"https://app.buildbuddy.io/"},"created an account")," and logged in."),Object(o.b)("p",null,"These URLs can be added directly to your ",Object(o.b)("inlineCode",{parentName:"p"},".bazelrc")," as long as no one outside of your organization has access to your source code."),Object(o.b)("p",null,"If people outside of your organization have access to your source code (open source projects, etc) - you'll want to pull your credentials into a separate file that is only accessible by members of your organization and/or your CI machines. Alternatively, you can store your API key in an environment variable / secret and pass these flags in manually or with a wrapper script."),Object(o.b)("h3",{id:"separate-auth-file"},"Separate auth file"),Object(o.b)("p",null,"Using the ",Object(o.b)("inlineCode",{parentName:"p"},"try-import")," directive in your ",Object(o.b)("inlineCode",{parentName:"p"},".bazelrc")," - you can direct bazel to pull in additional bazel configuration flags from a different file if the file exists (if the file does not exist, this directive will be ignored)."),Object(o.b)("p",null,"You can then place a second ",Object(o.b)("inlineCode",{parentName:"p"},"auth.bazelrc")," file in a location that's only accessible to members of your organization:"),Object(o.b)("pre",null,Object(o.b)("code",{parentName:"pre"},"build --bes_backend=grpcs://YOUR_API_KEY@cloud.buildbuddy.io\nbuild --remote_cache=grpcs://YOUR_API_KEY@cloud.buildbuddy.io\nbuild --remote_executor=grpcs://YOUR_API_KEY@cloud.buildbuddy.io\n")),Object(o.b)("p",null,"And add a ",Object(o.b)("inlineCode",{parentName:"p"},"try-import")," to your main ",Object(o.b)("inlineCode",{parentName:"p"},".bazelrc")," file at the root of your ",Object(o.b)("inlineCode",{parentName:"p"},"WORKSPACE"),":"),Object(o.b)("pre",null,Object(o.b)("code",{parentName:"pre"},"try-import /path/to/your/auth.bazelrc\n")),Object(o.b)("h3",{id:"command-line"},"Command line"),Object(o.b)("p",null,"The command line method allows you to store your API key in an environment variable or Github secret, and then pass authenticated flags in either manually or with a wrapper script."),Object(o.b)("p",null,"If using Github secrets - you can create a secret called ",Object(o.b)("inlineCode",{parentName:"p"},"BUILDBUDDY_API_KEY")," containing your API key, then use that in your workflows:"),Object(o.b)("pre",null,Object(o.b)("code",{parentName:"pre"},"bazel build --config=remote --bes_backend=${BUILDBUDDY_API_KEY}@cloud.buildbuddy.io --remote_cache=${BUILDBUDDY_API_KEY}@cloud.buildbuddy.io --remote_executor=${BUILDBUDDY_API_KEY}@cloud.buildbuddy.io\n")),Object(o.b)("h2",{id:"certificate"},"Certificate"),Object(o.b)("p",null,"The other option for authenticating your BuildBuddy requests is with certificates. Your BuildBuddy certificates can be used by anyone in your organization, as it ties builds to your org - not your individual user."),Object(o.b)("p",null,"You can download these certificates in your ",Object(o.b)("a",{parentName:"p",href:"https://app.buildbuddy.io/docs/setup/"},"setup instructions")," once you've ",Object(o.b)("a",{parentName:"p",href:"http://app.buildbuddy.io/"},"created an account")," and logged in. You'll first need to select ",Object(o.b)("inlineCode",{parentName:"p"},"Certificate")," as your auth option, then click ",Object(o.b)("inlineCode",{parentName:"p"},"Download buildbuddy-cert.pem")," and ",Object(o.b)("inlineCode",{parentName:"p"},"Download buildbuddy-key.pem"),"."),Object(o.b)("p",null,"Once you've downloaded your cert and key files - you can place them in a location that's only accessible to members of your organization and/or your CI machines."),Object(o.b)("p",null,"You can then add the following lines to your ",Object(o.b)("inlineCode",{parentName:"p"},".bazelrc"),":"),Object(o.b)("pre",null,Object(o.b)("code",{parentName:"pre"},"build --tls_client_certificate=/path/to/your/buildbuddy-cert.pem\nbuild --tls_client_key=/path/to/your/buildbuddy-key.pem\n")),Object(o.b)("p",null,"Make sure to update the paths to point to the location in which you've placed the files. If placing them in your workspace root, you can simply do:"),Object(o.b)("pre",null,Object(o.b)("code",{parentName:"pre"},"build --tls_client_certificate=buildbuddy-cert.pem\nbuild --tls_client_key=buildbuddy-key.pem\n")))}d.isMDXComponent=!0},128:function(e,t,n){"use strict";n.d(t,"a",(function(){return b})),n.d(t,"b",(function(){return y}));var i=n(0),a=n.n(i);function o(e,t,n){return t in e?Object.defineProperty(e,t,{value:n,enumerable:!0,configurable:!0,writable:!0}):e[t]=n,e}function r(e,t){var n=Object.keys(e);if(Object.getOwnPropertySymbols){var i=Object.getOwnPropertySymbols(e);t&&(i=i.filter((function(t){return Object.getOwnPropertyDescriptor(e,t).enumerable}))),n.push.apply(n,i)}return n}function u(e){for(var t=1;t<arguments.length;t++){var n=null!=arguments[t]?arguments[t]:{};t%2?r(Object(n),!0).forEach((function(t){o(e,t,n[t])})):Object.getOwnPropertyDescriptors?Object.defineProperties(e,Object.getOwnPropertyDescriptors(n)):r(Object(n)).forEach((function(t){Object.defineProperty(e,t,Object.getOwnPropertyDescriptor(n,t))}))}return e}function c(e,t){if(null==e)return{};var n,i,a=function(e,t){if(null==e)return{};var n,i,a={},o=Object.keys(e);for(i=0;i<o.length;i++)n=o[i],t.indexOf(n)>=0||(a[n]=e[n]);return a}(e,t);if(Object.getOwnPropertySymbols){var o=Object.getOwnPropertySymbols(e);for(i=0;i<o.length;i++)n=o[i],t.indexOf(n)>=0||Object.prototype.propertyIsEnumerable.call(e,n)&&(a[n]=e[n])}return a}var l=a.a.createContext({}),d=function(e){var t=a.a.useContext(l),n=t;return e&&(n="function"==typeof e?e(t):u(u({},t),e)),n},b=function(e){var t=d(e.components);return a.a.createElement(l.Provider,{value:t},e.children)},p={inlineCode:"code",wrapper:function(e){var t=e.children;return a.a.createElement(a.a.Fragment,{},t)}},s=a.a.forwardRef((function(e,t){var n=e.components,i=e.mdxType,o=e.originalType,r=e.parentName,l=c(e,["components","mdxType","originalType","parentName"]),b=d(n),s=i,y=b["".concat(r,".").concat(s)]||b[s]||p[s]||o;return n?a.a.createElement(y,u(u({ref:t},l),{},{components:n})):a.a.createElement(y,u({ref:t},l))}));function y(e,t){var n=arguments,i=t&&t.mdxType;if("string"==typeof e||i){var o=n.length,r=new Array(o);r[0]=s;var u={};for(var c in t)hasOwnProperty.call(t,c)&&(u[c]=t[c]);u.originalType=e,u.mdxType="string"==typeof e?e:i,r[1]=u;for(var l=2;l<o;l++)r[l]=n[l];return a.a.createElement.apply(null,r)}return a.a.createElement.apply(null,n)}s.displayName="MDXCreateElement"}}]);